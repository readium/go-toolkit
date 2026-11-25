package lcp_test

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/protection/lcp"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Protects an EPUB the way lcpencrypt does, computing the size and checksum
// the License Server expects along with the content key.
func ExampleEncryptEPUB() {
	ctx := context.Background()
	key, err := lcp.GenerateContentKey()
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("protected.epub")
	if err != nil {
		log.Fatal(err)
	}

	hash := sha256.New()
	counter := &countingWriter{}
	w := io.MultiWriter(out, hash, counter)

	a := asset.File(url.MustURLFromString("book.epub"))
	if err := lcp.EncryptEPUB(ctx, a, key, w, lcp.Options{}); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("content key:", key.Base64())
	fmt.Println("size:", counter.n, "sha256:", hex.EncodeToString(hash.Sum(nil)))
}

// Encrypts the HTML resources of a Readium Web Publication and streams one encrypted resource.
func ExampleEncryptPublication() {
	ctx := context.Background()
	key, err := lcp.GenerateContentKey()
	if err != nil {
		log.Fatal(err)
	}

	publication, err := streamer.New(streamer.Config{}).Open(ctx, asset.File(url.MustURLFromString("book.webpub")), "")
	if err != nil {
		log.Fatal(err)
	}
	defer publication.Close()

	protected, err := lcp.EncryptPublication(ctx, publication, key, lcp.Options{
		// Leave images in the clear.
		Encrypt: func(link manifest.Link) bool {
			return link.MediaType != nil && link.MediaType.IsHTML()
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, link := range protected.Manifest.ReadingOrder {
		if !lcp.IsLCPEncrypted(link.Properties.Encryption()) {
			continue
		}
		resource := protected.Get(ctx, link)
		defer resource.Close()
		if _, rerr := resource.Stream(ctx, os.Stdout, 0, 0); rerr != nil {
			log.Fatal(rerr)
		}
		break
	}
}

// Opens an LCP-protected publication whose content key is known, and reads
// its decrypted resources.
func ExampleDecryptBuilder() {
	ctx := context.Background()
	var key lcp.ContentKey // retrieved from the License Server

	publication, err := streamer.New(streamer.Config{
		OnCreatePublication: func(b *pub.Builder) error {
			return lcp.DecryptBuilder(b, key)
		},
	}).Open(ctx, asset.File(url.MustURLFromString("protected.epub")), "")
	if err != nil {
		log.Fatal(err)
	}
	defer publication.Close()

	first := publication.Manifest.ReadingOrder[0]
	content, rerr := publication.Get(ctx, first).Read(ctx, 0, 0)
	if rerr != nil {
		log.Fatal(rerr)
	}
	fmt.Println(len(content))
}

// Opens LCP-protected publications in the streamer with the user passphrase,
// after checking the license signature, provider certificate and rights.
func ExampleContentProtection() {
	ctx := context.Background()

	pemData, err := os.ReadFile("lcp-root-ca.pem")
	if err != nil {
		log.Fatal(err)
	}
	block, _ := pem.Decode(pemData)
	root, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)

	// The certificate revocation list of the root, kept current.
	crlData, err := os.ReadFile("lcp-root.crl")
	if err != nil {
		log.Fatal(err)
	}
	crl, err := x509.ParseRevocationList(crlData)
	if err != nil {
		log.Fatal(err)
	}
	if err := crl.CheckSignatureFrom(root); err != nil {
		log.Fatal(err)
	}

	s := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{lcp.ContentProtection{
			Verify: lcp.Verifier{Roots: roots, IsRevoked: lcp.RevokedBy(crl)}.Verify,
		}},
	})
	publication, err := s.Open(ctx, asset.File(url.MustURLFromString("protected.epub")), "user passphrase")
	if errors.Is(err, lcp.ErrWrongUserKey) {
		log.Fatal("wrong passphrase")
	} else if err != nil {
		log.Fatal(err)
	}
	defer publication.Close()
}

// Decrypts an LCP-protected EPUB with its embedded license, given the user
// passphrase.
func ExampleLicense_ContentKeyFromPassphrase() {
	ctx := context.Background()
	a := asset.File(url.MustURLFromString("protected.epub"))

	license, err := lcp.ReadLicense(ctx, a, lcp.Options{})
	if err != nil {
		log.Fatal(err)
	}
	contentKey, err := license.ContentKeyFromPassphrase("user passphrase")
	if err != nil {
		log.Fatal(err)
	}

	out, err := os.Create("unprotected.epub")
	if err != nil {
		log.Fatal(err)
	}
	if err := lcp.DecryptEPUB(ctx, a, contentKey, out, lcp.Options{}); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
