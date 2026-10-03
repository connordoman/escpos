package escpos_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/connordoman/escpos"
)

func Example() {
	conn, err := escpos.OpenFile("/dev/usb/lp0") // or usb.Open, escpos.DialTCP, serial.Open
	if err != nil {
		log.Fatal(err)
	}
	p := escpos.New(conn)
	defer p.Close()

	p.Initialize()
	p.SetAlign(escpos.AlignCenter)
	p.SetCharacterSize(2, 2)
	p.Textln("RECEIPT")
	p.SetCharacterSize(1, 1)
	p.SetAlign(escpos.AlignLeft)
	p.HorizontalRule('─')
	p.Columns("Coffee", "$4.50")
	p.Columns("Muffin", "$3.25")
	p.SetEmphasis(true)
	p.Columns("Total", "$7.75")
	p.SetEmphasis(false)
	p.SetAlign(escpos.AlignCenter)
	p.PrintQRCode("https://example.com/r/1234", 0, escpos.QRErrorM, 6)
	p.FeedAndCut(0)

	if _, err := p.Flush(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// Jobs can be built per request without holding the printer, then sent with
// Send, which serializes access to the device.
func ExamplePrinter_Send() {
	conn, err := escpos.DialTCP(context.Background(), "192.168.1.87")
	if err != nil {
		log.Fatal(err)
	}
	p := escpos.New(conn)
	defer p.Close()

	http.HandleFunc("POST /print", func(w http.ResponseWriter, r *http.Request) {
		job := escpos.NewBuilder(escpos.PaperWidth80mm)
		job.Initialize()
		job.Textln(r.FormValue("text"))
		job.FeedAndCut(0)

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if _, err := p.Send(ctx, job); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func ExamplePrinter_Status() {
	conn, err := escpos.OpenFile("/dev/usb/lp0")
	if err != nil {
		log.Fatal(err)
	}
	p := escpos.New(conn)
	defer p.Close()

	s, err := p.Status(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("cover open:", s.Offline.CoverOpen(), "paper out:", s.Paper.PaperEnd(), "ready:", s.Ready())
}

func ExampleBuilder_SetProcessIDResponse() {
	conn, err := escpos.OpenFile("/dev/usb/lp0")
	if err != nil {
		log.Fatal(err)
	}
	p := escpos.New(conn)
	defer p.Close()

	id := [4]byte{'J', '0', '0', '1'}
	p.Textln("Hello")
	p.FeedAndCut(0)
	p.SetProcessIDResponse(id)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := p.Flush(ctx); err != nil {
		log.Fatal(err)
	}
	// Returns once the printer has processed everything before the ID.
	if err := p.WaitProcessID(ctx, id); err != nil {
		log.Fatal(err)
	}
}
