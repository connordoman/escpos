package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/connordoman/escpos"
	"github.com/spf13/cobra"
)

func newStatusCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Query the printer's status and identity",
		Long: `Query the real-time status (DLE EOT), paper sensor (GS r) and printer
information (GS I). This needs a connection that can be read from.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.dryRun || opts.output != "" {
				return errors.New("status needs a printer; remove --dry-run/--output")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			conn, err := opts.connect(ctx)
			if err != nil {
				return err
			}
			p := escpos.New(conn, escpos.WithTimeout(2*time.Second))
			defer p.Close()
			return printStatus(ctx, cmd.OutOrStdout(), p)
		},
	}
}

func printStatus(ctx context.Context, out io.Writer, p *escpos.Printer) error {
	s, err := p.Status(ctx)
	if errors.Is(err, escpos.ErrNotReadable) {
		return fmt.Errorf("this connection cannot be read from: %w", err)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	if err != nil {
		fmt.Fprintf(tw, "Real-time status:\terror: %v\n", err)
	} else {
		fmt.Fprintf(tw, "Ready:\t%s\n", yesNo(s.Ready()))
		fmt.Fprintf(tw, "Cover open:\t%s\n", yesNo(s.Offline.CoverOpen()))
		fmt.Fprintf(tw, "Paper end:\t%s\n", yesNo(s.Paper.PaperEnd()))
		fmt.Fprintf(tw, "Feed button pressed:\t%s\n", yesNo(s.Offline.FeedButtonPressed()))
		fmt.Fprintf(tw, "Error:\t%s\n", yesNo(s.Offline.Error()))
		fmt.Fprintf(tw, "  Auto cutter error:\t%s\n", yesNo(s.Error.AutoCutterError()))
		fmt.Fprintf(tw, "  Unrecoverable error:\t%s\n", yesNo(s.Error.UnrecoverableError()))
		fmt.Fprintf(tw, "  Auto-recoverable error:\t%s\n", yesNo(s.Error.AutoRecoverableError()))
		fmt.Fprintf(tw, "Drawer signal high:\t%s\n", yesNo(s.Printer.DrawerSignalHigh()))
		fmt.Fprintf(tw, "Raw bytes:\t%02x %02x %02x %02x\n", byte(s.Printer), byte(s.Offline), byte(s.Error), byte(s.Paper))
	}

	if ps, err := p.PaperSensorStatus(ctx); err != nil {
		fmt.Fprintf(tw, "Paper near end:\terror: %v\n", err)
	} else {
		fmt.Fprintf(tw, "Paper near end:\t%s\n", yesNo(ps.NearEnd()))
	}

	if id, err := p.PrinterID(ctx, escpos.PrinterIDModel); err != nil {
		fmt.Fprintf(tw, "Model ID:\terror: %v\n", err)
	} else {
		fmt.Fprintf(tw, "Model ID:\t%#02x\n", id)
	}
	if id, err := p.PrinterID(ctx, escpos.PrinterIDTypeID); err != nil {
		fmt.Fprintf(tw, "Type ID:\terror: %v\n", err)
	} else {
		t := escpos.TypeID(id)
		fmt.Fprintf(tw, "Type ID:\t%#02x (auto cutter: %s, multi-byte: %s)\n", id, yesNo(t.AutoCutter()), yesNo(t.MultiByte()))
	}
	// Automatic Status Back: enabling it makes the printer send a 4-byte
	// report straight away. Disable it again afterwards so later queries
	// don't read stray reports.
	p.SetAutoStatusBack(escpos.ASBErrorStatus | escpos.ASBPaperSensor)
	if _, err := p.Flush(ctx); err != nil {
		fmt.Fprintf(tw, "ASB report:	error: %v\n", err)
	} else if r, err := p.ReadASB(ctx); err != nil {
		fmt.Fprintf(tw, "ASB report:	error: %v\n", err)
	} else {
		fmt.Fprintf(tw, "ASB report:	% x\n", r[:])
	}
	p.SetAutoStatusBack(0)
	p.Flush(ctx)

	for _, info := range []struct {
		t    escpos.PrinterIDType
		name string
	}{
		{escpos.PrinterInfoFirmware, "Firmware"},
		{escpos.PrinterInfoManufacturer, "Manufacturer"},
		{escpos.PrinterInfoName, "Printer name"},
		{escpos.PrinterInfoSerial, "Serial number"},
		{escpos.PrinterInfoFonts, "Additional fonts"},
	} {
		if v, err := p.PrinterInfo(ctx, info.t); err != nil {
			fmt.Fprintf(tw, "%s:\terror: %v\n", info.name, err)
		} else {
			fmt.Fprintf(tw, "%s:\t%s\n", info.name, v)
		}
	}
	return nil
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func newListCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List detected USB printers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			osDriver, libusb, osErr, libErr := listUSB()
			printList(out, "OS printer driver (use with --device)", osDriver, osErr)
			fmt.Fprintln(out)
			printList(out, "libusb (use with --libusb --vid/--pid)", libusb, libErr)
			return nil
		},
	}
}

func printList(out io.Writer, title string, printers []escpos.USBPrinter, err error) {
	fmt.Fprintln(out, title+":")
	switch {
	case err != nil:
		fmt.Fprintf(out, "  unavailable: %v\n", err)
	case len(printers) == 0:
		fmt.Fprintln(out, "  none found")
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, p := range printers {
		name := p.Manufacturer + " " + p.Product
		if p.Manufacturer == "" && p.Product == "" {
			name = "-"
		}
		fmt.Fprintf(tw, "  %04x:%04x\t%s\tserial %s\t%s\n", p.VendorID, p.ProductID, name, orDash(p.Serial), p.Path)
	}
	tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func newDrawerCommand(opts *options) *cobra.Command {
	var pin int
	var realtime bool
	var pulse uint8
	cmd := &cobra.Command{
		Use:   "drawer",
		Short: "Open the cash drawer connected to the RJ11 port",
		Long: `Pulse the cash drawer kick-out connector. By default ESC p is used (100 ms
on); --realtime uses DLE DC4, which works even while the printer is busy or
disabled.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var drawer escpos.DrawerPin
			switch pin {
			case 2:
				drawer = escpos.DrawerPin2
			case 5:
				drawer = escpos.DrawerPin5
			default:
				return fmt.Errorf("--pin must be 2 or 5, not %d", pin)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()
			conn, err := opts.connect(ctx)
			if err != nil {
				return err
			}
			p := escpos.New(conn)
			defer p.Close()
			if realtime {
				return p.RealTimePulse(ctx, drawer, pulse)
			}
			if err := p.OpenCashDrawer(drawer); err != nil {
				return err
			}
			_, err = p.Flush(ctx)
			return err
		},
	}
	cmd.Flags().IntVar(&pin, "pin", 2, "drawer connector pin: 2 (first drawer) or 5 (second)")
	cmd.Flags().BoolVar(&realtime, "realtime", false, "use the real-time pulse command (DLE DC4)")
	cmd.Flags().Uint8Var(&pulse, "pulse", 1, "real-time pulse length in 100 ms units (1-8)")
	return cmd
}
