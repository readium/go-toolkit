package decoder

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/readium/r2-streamer-go/pkg/pub"
	. "github.com/smartystreets/goconvey/convey"
)

var testPublication pub.Publication
var testFonts []byte

func init() {

	//	testPublication, _ = parser.Parse("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL")
	testPublication.Metadata.Identifier = "urn:uuid:36d5078e-ff7d-468e-a5f3-f47c14b91f2f"
	ft, _ := os.Open("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL/fonts/cut-cut.woff")
	testFonts, _ = ioutil.ReadAll(ft)
}

func TestAdobeFonts(t *testing.T) {

	f, _ := os.Open("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL/fonts/cut-cut.adb.woff")

	Convey("Given cut-cut.adb.woff fonts", t, func() {
		fd, _ := DecodeAdobeFont(&testPublication, pub.Link{}, f)
		buff, _ := ioutil.ReadAll(fd)
		Convey("The adobe fonts is deobfuscated", func() {
			So(bytes.Equal(buff, testFonts), ShouldBeTrue)
		})

	})

}

func TestIdpfFonts(t *testing.T) {

	f, _ := os.Open("../test/readium-test-files/functional/smoke-tests/SmokeTestFXL/fonts/cut-cut.obf.woff")

	Convey("Given cut-cut.obf.woff fonts", t, func() {
		fd, _ := DecodeIdpfFont(&testPublication, pub.Link{}, f)
		buff, _ := ioutil.ReadAll(fd)
		Convey("The idpf fonts is deobfuscated", func() {
			So(bytes.Equal(buff, testFonts), ShouldBeTrue)
		})

	})

}
