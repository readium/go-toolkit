package parser

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

// HCFC = hrefCommonFirstComponent abbreviated

func TestHCFCEmptyWhenFilesInRoot(t *testing.T) {
	assert.Equal(t, "", hrefCommonFirstComponent(manifest.LinkList{
		{Href: manifest.MustNewHREFFromString("im1.jpg", false)},
		{Href: manifest.MustNewHREFFromString("im2.jpg", false)},
		{Href: manifest.MustNewHREFFromString("toc.xml", false)},
	}), "hrefCommonFirstComponent is empty when files are in the root")
}

func TestHCFCEmptyWhenFilesInDifferentDirs(t *testing.T) {
	assert.Equal(t, "", hrefCommonFirstComponent(manifest.LinkList{
		{Href: manifest.MustNewHREFFromString("dir1/im1.jpg", false)},
		{Href: manifest.MustNewHREFFromString("dir2/im2.jpg", false)},
		{Href: manifest.MustNewHREFFromString("toc.xml", false)},
	}), "hrefCommonFirstComponent is empty when files are in different directories")
}

func TestHCFCCorrectWhenSameDir(t *testing.T) {
	assert.Equal(t, "root", hrefCommonFirstComponent(manifest.LinkList{
		{Href: manifest.MustNewHREFFromString("root/im1.jpg", false)},
		{Href: manifest.MustNewHREFFromString("root/im2.jpg", false)},
		{Href: manifest.MustNewHREFFromString("root/xml/toc.xml", false)},
	}), "hrefCommonFirstComponent is empty when files are in different directories")
}
