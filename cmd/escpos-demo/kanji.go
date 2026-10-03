package main

import (
	"math"

	"github.com/connordoman/escpos"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// kanjiSection prints double-byte text in Kanji mode, encoded as GBK. It
// needs Chinese firmware: printers without it report TypeID.MultiByte() as
// false and print the GBK bytes as single-byte characters. The RP326 tested
// (firmware 7.03) is one of those.
func kanjiSection(b *escpos.Builder) error {
	var c check
	b.Textln("Kanji mode (FS &) with GBK text:")
	b.SelectKanjiMode()
	b.SetTextEncoder(simplifiedchinese.GBK.NewEncoder())

	b.Textln("你好，世界！中文打印测试")
	b.Textln("日本語：こんにちは、カタカナ")
	b.Textln("繁體字：漢字　价格：￥28.50")

	b.SetKanjiPrintMode(escpos.KanjiDoubleWidth | escpos.KanjiDoubleHeight)
	b.Textln("倍宽倍高")
	b.SetKanjiPrintMode(0)
	b.SetKanjiQuadruple(true)
	b.Textln("四倍")
	b.SetKanjiQuadruple(false)
	c.do(b.SetKanjiUnderline(escpos.UnderlineSingle))
	b.Textln("下划线")
	c.do(b.SetKanjiUnderline(escpos.UnderlineOff))
	b.SetKanjiSpacing(6, 6)
	b.Textln("字间距")
	b.SetKanjiSpacing(0, 0)

	// A user-defined character (FS 2) at code FE A1.
	_, _, glyph := escpos.NewBitmap(kanjiGlyph(), escpos.ImageOptions{}).Columns()
	c.do(b.DefineKanjiCharacter(0xFE, 0xA1, glyph))
	b.Text("自定义字符：")
	b.Raw(0xFE, 0xA1, 0xFE, 0xA1, 0xFE, 0xA1)
	b.LineFeed()

	b.SetTextEncoder(nil)
	b.CancelKanjiMode()
	return c.err
}

// kanjiGlyph draws a 24×24 placeholder character: a framed star.
func kanjiGlyph() *grayImage {
	return paint(24, 24, func(x, y float64) bool {
		if x < 2 || y < 2 || x > 22 || y > 22 {
			return true
		}
		// Five-pointed star in polar form.
		dx, dy := x-12, y-12.5
		r := math.Hypot(dx, dy)
		a := math.Atan2(dy, dx) + math.Pi/2
		k := math.Mod(a+2*math.Pi, 2*math.Pi/5) / (2 * math.Pi / 5)
		edge := 4 + 4.5*math.Abs(1-2*k)
		return r < edge
	})
}
