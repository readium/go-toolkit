package parser

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

// HCFC = hrefCommonFirstComponent abbreviated

func TestHCFCEmptyWhenFilesInRoot(t *testing.T) {
	assert.Equal(t, "", hrefCommonFirstComponent(manifest.LinkList{
		{Href: "/im1.jpg"},
		{Href: "/im2.jpg"},
		{Href: "/toc.xml"},
	}), "hrefCommonFirstComponent is empty when files are in the root")
}

func TestHCFCEmptyWhenFilesInDifferentDirs(t *testing.T) {
	assert.Equal(t, "", hrefCommonFirstComponent(manifest.LinkList{
		{Href: "/dir1/im1.jpg"},
		{Href: "/dir2/im2.jpg"},
		{Href: "/toc.xml"},
	}), "hrefCommonFirstComponent is empty when files are in different directories")
}

func TestHCFCCorrectWhenSameDir(t *testing.T) {
	assert.Equal(t, "root", hrefCommonFirstComponent(manifest.LinkList{
		{Href: "/root/im1.jpg"},
		{Href: "/root/im2.jpg"},
		{Href: "/root/xml/toc.xml"},
	}), "hrefCommonFirstComponent is empty when files are in different directories")
}
