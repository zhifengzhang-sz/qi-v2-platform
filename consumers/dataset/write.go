package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
)

// repository is the repository the data sets are made and locked in, the image's source label: on GHCR it links the
// image's package to it.
const repository = "https://github.com/zhifengzhang-sz/qi-v2-platform"

// The OCI image specification's media types of the layout's blobs.
const (
	mediaIndex    = "application/vnd.oci.image.index.v1+json"
	mediaManifest = "application/vnd.oci.image.manifest.v1+json"
	mediaConfig   = "application/vnd.oci.image.config.v1+json"
	mediaLayer    = "application/vnd.oci.image.layer.v1.tar+gzip"
)

// file is one file of a data set: its path in the image, its bytes, its K-lines.
type file struct {
	path   string
	data   []byte
	klines int
}

// files is the data set's files: one a calendar month of its sessions' dates in the zone, YYYY-MM.jsonl, in the order
// of their names, each its K-lines in the order of their dates, one a line, market 1.1.0's K-line in market's canonical
// text (raw.FromBase), a line feed after each; each K-line's index its place in the data set, from 0.
func files(ks []market.KLine, s spec) ([]file, error) {
	var set []file
	for i, k := range ks {
		k.Index = uint64(i)
		t, err := raw.FromBase(k)
		if err != nil {
			return nil, fmt.Errorf("K-line %d of the data set: %w", i, err)
		}
		month := session(k, s.loc)[:len("2006-01")] + ".jsonl"
		if len(set) == 0 || set[len(set)-1].path != month {
			set = append(set, file{path: month})
		}
		f := &set[len(set)-1]
		f.data = append(append(f.data, t.Text()...), '\n')
		f.klines++
	}
	return set, nil
}

// descriptor is the OCI image specification's: a blob's media type, digest and size, and annotations.
type descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Config        descriptor        `json:"config"`
	Layers        []descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations"`
}

type imageConfig struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Config       struct {
		Labels map[string]string `json:"Labels"`
	} `json:"config"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

type index struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []descriptor `json:"manifests"`
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// layer is a file's layer: a tar of the file alone at the image's root, mode 0444, owner 0:0, its time 0, then
// gzip-compressed with no time and no name in its header; and the tar's digest, the config's diff_id.
func layer(f file) ([]byte, string, error) {
	var tb bytes.Buffer
	tw := tar.NewWriter(&tb)
	h := &tar.Header{Typeflag: tar.TypeReg, Name: f.path, Size: int64(len(f.data)), Mode: 0o444,
		ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}
	if err := tw.WriteHeader(h); err != nil {
		return nil, "", fmt.Errorf("layer %s: %w", f.path, err)
	}
	if _, err := tw.Write(f.data); err != nil {
		return nil, "", fmt.Errorf("layer %s: %w", f.path, err)
	}
	if err := tw.Close(); err != nil {
		return nil, "", fmt.Errorf("layer %s: %w", f.path, err)
	}
	var gb bytes.Buffer
	zw, err := gzip.NewWriterLevel(&gb, gzip.BestCompression)
	if err != nil {
		return nil, "", err
	}
	if _, err := zw.Write(tb.Bytes()); err != nil {
		return nil, "", fmt.Errorf("layer %s: %w", f.path, err)
	}
	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("layer %s: %w", f.path, err)
	}
	return gb.Bytes(), digest(tb.Bytes()), nil
}

// labels are the image's: its source, title, version and description, its config's labels and its manifest's
// annotations alike.
func labels(s spec) map[string]string {
	return map[string]string{
		"org.opencontainers.image.source":  repository,
		"org.opencontainers.image.title":   s.name,
		"org.opencontainers.image.version": s.version,
		"org.opencontainers.image.description": fmt.Sprintf("K-lines of %s, its sessions %s to %s in %s: a file a "+
			"month, a line each market 1.1.0's K-line", s.topic, s.from, s.to, s.zone),
	}
}

// layout writes the image as an OCI image layout into dir: oci-layout; blobs/sha256/, each layer (one a file, in the
// order of the files), the config (linux/amd64, the labels, the layers' diff_ids, no command and no time) and the
// manifest (the config, the layers, the labels as its annotations); index.json, the manifest named by its tag, the
// version. It is the image's digest, its manifest's.
func layout(dir string, set []file, s spec) (string, error) {
	blobs := filepath.Join(dir, "blobs", "sha256")
	if err := os.MkdirAll(blobs, 0o755); err != nil {
		return "", err
	}
	blob := func(media string, b []byte) (descriptor, error) {
		d := descriptor{MediaType: media, Digest: digest(b), Size: int64(len(b))}
		return d, os.WriteFile(filepath.Join(blobs, d.Digest[len("sha256:"):]), b, 0o644)
	}
	var cfg imageConfig
	cfg.Architecture, cfg.OS = "amd64", "linux"
	cfg.Config.Labels = labels(s)
	cfg.RootFS.Type = "layers"
	m := manifest{SchemaVersion: 2, MediaType: mediaManifest, Annotations: labels(s)}
	for _, f := range set {
		gz, diff, err := layer(f)
		if err != nil {
			return "", err
		}
		d, err := blob(mediaLayer, gz)
		if err != nil {
			return "", err
		}
		m.Layers = append(m.Layers, d)
		cfg.RootFS.DiffIDs = append(cfg.RootFS.DiffIDs, diff)
	}
	cb, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if m.Config, err = blob(mediaConfig, cb); err != nil {
		return "", err
	}
	mb, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	md, err := blob(mediaManifest, mb)
	if err != nil {
		return "", err
	}
	md.Annotations = map[string]string{"org.opencontainers.image.ref.name": s.version}
	ib, err := json.Marshal(index{SchemaVersion: 2, MediaType: mediaIndex, Manifests: []descriptor{md}})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), ib, 0o644); err != nil {
		return "", err
	}
	return md.Digest, os.WriteFile(filepath.Join(dir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644)
}

// lockFile is a data set's lock (the design, Its form): what a version is made of (dataset, version, image, made) and
// what its making pins (tag, digest, files).
type lockFile struct {
	Dataset string      `json:"dataset"`
	Version string      `json:"version"`
	Image   string      `json:"image"`
	Tag     string      `json:"tag"`
	Digest  string      `json:"digest"`
	Made    made        `json:"made"`
	Files   []lockEntry `json:"files"`
}

type made struct {
	Topic string `json:"topic"`
	Zone  string `json:"zone"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type lockEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	KLines int    `json:"klines"`
}

// lock is the data set's lock: JSON, indented by two spaces, a line feed at its end.
func lock(set []file, s spec, image string) ([]byte, error) {
	l := lockFile{Dataset: s.name, Version: s.version, Image: s.image, Tag: s.version, Digest: image,
		Made: made{Topic: s.topic, Zone: s.zone, From: s.from, To: s.to}}
	for _, f := range set {
		sum := sha256.Sum256(f.data)
		l.Files = append(l.Files, lockEntry{Path: f.path, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(f.data)),
			KLines: f.klines})
	}
	b, err := json.MarshalIndent(l, "", "  ")
	return append(b, '\n'), err
}

// run makes the data set: the topic read (read), its range picked (pick), then dir made, which must not exist, and
// in it files/ (each file), oci/ (the image's layout) and <product>.lock; then a line to log of what it made.
func run(ctx context.Context, bus stream.Bus, s spec, dir string, log io.Writer) error {
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("-out %s: a folder that does not exist (%v)", dir, err)
	}
	all, t, err := read(ctx, bus, s.topic)
	if err != nil {
		return err
	}
	ks, err := pick(all, s)
	if err != nil {
		return err
	}
	set, err := files(ks, s)
	if err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(dir, "files"), 0o755); err != nil {
		return err
	}
	size := 0
	for _, f := range set {
		if err := os.WriteFile(filepath.Join(dir, "files", f.path), f.data, 0o644); err != nil {
			return err
		}
		size += len(f.data)
	}
	image, err := layout(filepath.Join(dir, "oci"), set, s)
	if err != nil {
		return err
	}
	l, err := lock(set, s, image)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, s.name+".lock"), l, 0o644); err != nil {
		return err
	}
	at := func(k market.KLine) string {
		return time.UnixMilli(int64(k.Date)).In(s.loc).Format("2006-01-02T15:04Z07:00")
	}
	fmt.Fprintf(log, "dataset: %s: %d K-lines read, %d copies dropped (%d of another value); %s %s: %s to %s, %d "+
		"K-lines in %d files, %d bytes; the image %s\n", s.topic, t.klines, t.copies, t.others, s.name, s.version,
		at(ks[0]), at(ks[len(ks)-1]), len(ks), len(set), size, image)
	return nil
}
