package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/cover"
)

type mode string

func (m mode) merge(dest, src cover.ProfileBlock) cover.ProfileBlock {
	if m == "set" {
		if src.Count != 0 {
			dest.Count = src.Count
		}
	} else {
		dest.Count += src.Count
	}
	return dest
}

type blockLocation struct {
	startLine, startCol, endLine, endCol int
}

func (l blockLocation) less(m blockLocation) bool {
	return l.startLine < m.startLine || l.startCol < m.startCol || l.endLine < m.endLine || l.endCol < m.endCol
}

type profile struct {
	fileName string
	blocks   map[blockLocation]cover.ProfileBlock
}

func (p *profile) merge(other *cover.Profile, mode mode) {
	for _, block := range other.Blocks {
		loc := blockLocation{block.StartLine, block.StartCol, block.EndLine, block.EndCol}
		if b, ok := p.blocks[loc]; ok {
			p.blocks[loc] = mode.merge(b, block)
		} else {
			p.blocks[loc] = block
		}
	}
}

func (p *profile) write(w io.Writer) error {
	locs := make([]blockLocation, 0, len(p.blocks))
	for loc := range p.blocks {
		locs = append(locs, loc)
	}
	sort.Slice(locs, func(i, j int) bool { return locs[i].less(locs[j]) })

	for _, l := range locs {
		b := p.blocks[l]
		if _, err := fmt.Fprintf(w, "%v:%v.%v,%v.%v %v %v\n",
			p.fileName, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt, b.Count); err != nil {
			return err
		}
	}
	return nil
}

type profiles struct {
	mode  mode
	files map[string]*profile
}

func (ps *profiles) merge(other []*cover.Profile) error {
	for _, o := range other {
		switch {
		case ps.mode == "":
			ps.mode = mode(o.Mode)
		case o.Mode != string(ps.mode):
			return fmt.Errorf("%v's coverage mode '%v' does not match the merged mode '%v'", o.FileName, o.Mode, ps.mode)
		}

		p, ok := ps.files[o.FileName]
		if !ok {
			p = &profile{fileName: o.FileName, blocks: map[blockLocation]cover.ProfileBlock{}}
			ps.files[o.FileName] = p
		}
		p.merge(o, ps.mode)
	}
	return nil
}

func (ps *profiles) write(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "mode: %v\n", ps.mode); err != nil {
		return err
	}

	files := make([]string, 0, len(ps.files))
	for fileName := range ps.files {
		files = append(files, fileName)
	}
	sort.Strings(files)

	for _, fileName := range files {
		if err := ps.files[fileName].write(w); err != nil {
			return err
		}
	}
	return nil
}

func fatalf(m string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, m+"\n", args...)
	os.Exit(1)
}

func main() {
	inPath := flag.String("in", "", "the path to the directory containing coverage data to merge")
	outPath := flag.String("out", "", "the path to the output file")
	flag.Parse()

	if *inPath == "" {
		fatalf("the -in flag is required")
	}

	profiles := profiles{files: map[string]*profile{}}

	entries, err := os.ReadDir(*inPath)
	if err != nil {
		fatalf("reading input: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".cov" {
			path := filepath.Join(*inPath, e.Name())
			rawProfiles, err := cover.ParseProfiles(path)
			if err != nil {
				fatalf("parsing profiles from '%v': %v", path, err)
			}
			if err = profiles.merge(rawProfiles); err != nil {
				fatalf("merging coverage from '%v': %v", path, err)
			}
		}
	}

	outFile := os.Stdout
	if *outPath != "" {
		outFile, err = os.Create(*outPath)
		if err != nil {
			fatalf("creating output file '%v': %v", *outPath)
		}
		defer outFile.Close()
	}

	if err = profiles.write(outFile); err != nil {
		fatalf("writing merged profile: %v", err)
	}
}
