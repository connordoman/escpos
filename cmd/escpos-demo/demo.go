package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/connordoman/escpos"
	"github.com/spf13/cobra"
)

// section is one part of the demo.
type section struct {
	name    string
	summary string
	// extra sections have side effects (sound, drawer, NV memory writes, a
	// printer reset) and only run when named explicitly.
	extra bool
	build func(b *escpos.Builder) error
}

var sections = []section{
	{name: "header", summary: "store header: sizes, reverse, alignment", build: headerSection},
	{name: "text", summary: "fonts, emphasis, underline, rotation, spacing", build: textSection},
	{name: "codepage", summary: "accented text, box drawing and code pages", build: codePageSection},
	{name: "receipt", summary: "itemised receipt: columns, tabs, margins", build: receiptSection},
	{name: "lipsum", summary: "word-wrapped paragraphs in both fonts", build: lipsumSection},
	{name: "barcodes", summary: "every 1D bar code symbology with HRI text", build: barcodeSection},
	{name: "2d", summary: "QR code and PDF417", build: twoDSection},
	{name: "images", summary: "raster, dithered, column and downloaded bit images", build: imageSection},
	{name: "chars", summary: "user-defined characters", build: userCharSection},
	{name: "pagemode", summary: "page mode layout in several directions", build: pageModeSection},
	{name: "counter", summary: "printer-side serial number counter", build: counterSection},
	{name: "macro", summary: "record a macro and replay it", build: macroSection},

	{name: "beep", summary: "sound the buzzer", extra: true, build: beepSection},
	{name: "drawer", summary: "kick the cash drawer (RJ11 port)", extra: true, build: drawerSection},
	{name: "kanji", summary: "double-byte text; needs Chinese firmware (not on the tested RP326)", extra: true, build: kanjiSection},
	{name: "nv", summary: "store and print an NV bit image (writes flash memory)", extra: true, build: nvSection},
	{name: "selftest", summary: "printer self-test page", extra: true, build: selfTestSection},
}

func findSection(name string) (section, bool) {
	i := slices.IndexFunc(sections, func(s section) bool { return s.name == name })
	if i < 0 {
		return section{}, false
	}
	return sections[i], true
}

func newDemoCommand(opts *options) *cobra.Command {
	var list, noCut, wait bool
	cmd := &cobra.Command{
		Use:   "demo [section...]",
		Short: "Print feature samples (all standard sections by default)",
		Long: `Print samples of the escpos package's features using placeholder data.

With no arguments every standard section is printed. Name sections to print
only those, in the order given; "all" expands to the standard sections.
Extra sections have side effects and only run when named.`,
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			names := []string{"all"}
			for _, s := range sections {
				names = append(names, s.name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				printSections(cmd.OutOrStdout())
				return nil
			}
			chosen, err := chooseSections(args)
			if err != nil {
				return err
			}
			job, err := buildJob(opts, chosen, !noCut)
			if err != nil {
				return err
			}
			return sendJob(cmd.Context(), opts, job, wait)
		},
	}
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list the available sections")
	cmd.Flags().BoolVar(&noCut, "no-cut", false, "do not feed and cut at the end")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait until the printer reports the job processed (GS ( H)")
	return cmd
}

func printSections(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SECTION\tDESCRIPTION")
	for _, s := range sections {
		summary := s.summary
		if s.extra {
			summary += " [extra]"
		}
		fmt.Fprintf(tw, "%s\t%s\n", s.name, summary)
	}
	tw.Flush()
}

func chooseSections(args []string) ([]section, error) {
	if len(args) == 0 {
		args = []string{"all"}
	}
	var chosen []section
	for _, name := range args {
		if name == "all" {
			for _, s := range sections {
				if !s.extra {
					chosen = append(chosen, s)
				}
			}
			continue
		}
		s, ok := findSection(name)
		if !ok {
			return nil, fmt.Errorf("unknown section %q (see demo --list)", name)
		}
		chosen = append(chosen, s)
	}
	return chosen, nil
}

// buildJob assembles the chosen sections into one job. Building happens
// before connecting, so invalid parameters are reported without printing
// anything.
func buildJob(opts *options, chosen []section, cut bool) (*escpos.Builder, error) {
	job := escpos.NewBuilder(opts.paperWidth)
	job.Initialize()
	for i, s := range chosen {
		if i > 0 {
			job.FeedLines(1)
		}
		sectionTitle(job, s.name)
		if err := s.build(job); err != nil {
			return nil, fmt.Errorf("section %s: %w", s.name, err)
		}
		// Each section leaves the printer in its default state.
		job.Initialize()
	}
	if cut {
		job.FeedAndCut(0)
	}
	return job, nil
}

// sectionTitle prints a reversed, full-width banner naming a section.
func sectionTitle(b *escpos.Builder, name string) {
	b.SetReverse(true)
	b.SetEmphasis(true)
	title := " " + strings.ToUpper(name)
	b.Textln(title + strings.Repeat(" ", max(b.CharactersPerLine()-len(title), 0)))
	b.SetReverse(false)
	b.SetEmphasis(false)
}

func sendJob(ctx context.Context, opts *options, job *escpos.Builder, wait bool) error {
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	var id [4]byte
	if wait {
		copy(id[:], fmt.Sprintf("%04d", time.Now().UnixMilli()%10000))
		if err := job.SetProcessIDResponse(id); err != nil {
			return err
		}
	}

	conn, err := opts.connect(ctx)
	if err != nil {
		return err
	}
	p := escpos.New(conn, escpos.WithPaperWidth(opts.paperWidth))
	defer p.Close()

	n, err := p.Send(ctx, job)
	if err != nil {
		return fmt.Errorf("sent %d of %d bytes: %w", n, job.Len(), err)
	}
	if opts.dryRun {
		return nil
	}
	fmt.Fprintf(os.Stderr, "sent %d bytes\n", n)
	if wait {
		fmt.Fprintln(os.Stderr, "waiting for the printer to finish...")
		if err := p.WaitProcessID(ctx, id); err != nil {
			return fmt.Errorf("waiting for process ID: %w", err)
		}
		fmt.Fprintln(os.Stderr, "printer reports the job processed")
	}
	return nil
}
