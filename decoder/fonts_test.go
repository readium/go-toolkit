package decoder

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/readium/r2-streamer-go/models"
	"github.com/readium/r2-streamer-go/parser"
	. "github.com/smartystreets/goconvey/convey"
)

var testPublication models.Publication
var testFonts []byte

func init() {

	testPublication, _ = parser.Parse("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL")
	ft, _ := os.Open("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL/fonts/cut-cut.woff")
	testFonts, _ = ioutil.ReadAll(ft)
}

func TestAdobeFonts(t *testing.T) {

	f, _ := os.Open("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL/fonts/cut-cut.adb.woff")

	Convey("Given cut-cut.adb.woff fonts", t, func() {
		fd, _ := DecodeAdobeFont(testPublication, models.Link{}, f)
		buff, _ := ioutil.ReadAll(fd)
		Convey("The adobe fonts is deobfuscated", func() {
			So(bytes.Equal(buff, testFonts), ShouldBeTrue)
		})

	})

}

func TestIdpfFonts(t *testing.T) {

	f, _ := os.Open("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL/fonts/cut-cut.obf.woff")

	Convey("Given cut-cut.obf.woff fonts", t, func() {
		fd, _ := DecodeIdpfFont(testPublication, models.Link{}, f)
		buff, _ := ioutil.ReadAll(fd)
		Convey("The idpf fonts is deobfuscated", func() {
			So(bytes.Equal(buff, testFonts), ShouldBeTrue)
		})

	})

}
