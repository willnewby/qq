package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleasePlatform(t *testing.T) {
	p, err := releasePlatform()
	require.NoError(t, err)
	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, "Darwin", p)
	case "linux":
		assert.Equal(t, "Linux", p)
	}
}

func TestReleaseArch(t *testing.T) {
	a, err := releaseArch()
	require.NoError(t, err)
	switch runtime.GOARCH {
	case "amd64":
		assert.Equal(t, "x86_64", a)
	case "arm64":
		assert.Equal(t, "arm64", a)
	}
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	content := []byte("fake-binary-content")

	// Build a tar.gz in memory with a "qq" entry
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "qq",
		Size:     int64(len(content)),
		Mode:     0755,
		Typeflag: tar.TypeReg,
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	data, err := extractBinaryFromTarGz(&buf, "qq")
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestExtractBinaryFromTarGz_NotFound(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	_, err := extractBinaryFromTarGz(&buf, "qq")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractBinaryFromTarGz_NestedPath(t *testing.T) {
	content := []byte("nested-binary")

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "./dist/qq",
		Size:     int64(len(content)),
		Mode:     0755,
		Typeflag: tar.TypeReg,
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	data, err := extractBinaryFromTarGz(&buf, "qq")
	require.NoError(t, err)
	assert.Equal(t, content, data)
}
