// Main package wraps sprite_sass tool for use with the command line
// See -h for list of available options
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"github.com/wellington/spritewell"
	"github.com/wellington/wellington/context"

	sprite "github.com/wellington/wellington"
)

const version = `v0.4.0`

var (
	Font, Dir, Gen, Input, Includes string
	MainFile, Style                 string
	Comments                        bool
	cpuprofile                      string
	Help, ShowVersion               bool
	BuildDir                        string
)

func init() {
	flag.BoolVar(&ShowVersion, "version", false, "Show the app version")

	flag.BoolVar(&Help, "help", false, "this help")
	flag.BoolVar(&Help, "h", false, "this help")

	flag.StringVar(&BuildDir, "b", "", "Build Directory")
	flag.StringVar(&Gen, "gen", ".", "Directory for generated images")

	flag.StringVar(&Includes, "p", "", "SASS import path")
	flag.StringVar(&Dir, "dir", "", "Image directory")
	flag.StringVar(&Dir, "d", "", "Image directory")
	flag.StringVar(&Font, "font", ".", "Font Directory")

	flag.StringVar(&Style, "style", "nested", "CSS nested style")
	flag.StringVar(&Style, "s", "nested", "CSS nested style")
	flag.BoolVar(&Comments, "comment", true, "Turn on source comments")
	flag.BoolVar(&Comments, "c", true, "Turn on source comments")

	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to file")
}

func main() {
	flag.Parse()

	if ShowVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// Profiling code
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Starting profiler")
		pprof.StartCPUProfile(f)
		defer func() {
			pprof.StopCPUProfile()
			err := f.Close()
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Stopping Profiller")
		}()
	}

	for _, v := range flag.Args() {
		if strings.HasPrefix(v, "-") {
			log.Fatalf("Please specify flags before other arguments: %s", v)
		}
	}

	if Help {
		fmt.Println("Please specify input filepath.")
		fmt.Println("\nAvailable options:")
		flag.PrintDefaults()
		return
	}

	if Gen != "" {
		err := os.MkdirAll(Gen, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}

	style, ok := context.Style[Style]

	if !ok {
		style = context.NESTED_STYLE
	}

	if len(flag.Args()) == 0 {
		// Read from stdin
		log.Print("Reading from stdin, -h for help")
		out := os.Stdout
		in := os.Stdin

		var pout bytes.Buffer
		ctx := context.Context{}
		_, err := startParser(&ctx, in, &pout, "")
		if err != nil {
			log.Println(err)
		}
		err = ctx.Compile(&pout, out)

		if err != nil {
			log.Println(err)
		}
	}

	SpriteCache := spritewell.SafeImageMap{
		M: make(map[string]spritewell.ImageList, 100)}
	ImageCache := spritewell.SafeImageMap{
		M: make(map[string]spritewell.ImageList, 100)}

	for _, f := range flag.Args() {
		// Remove partials
		if strings.HasPrefix(filepath.Base(f), "_") {
			continue
		}
		// log.Println("Open:", f)

		// If no imagedir specified, assume relative to the input file
		if Dir == "" {
			Dir = filepath.Dir(f)
		}
		var (
			out  io.WriteCloser
			fout string
		)
		if BuildDir != "" {
			// Build output file based off build directory and input filename
			rel, _ := filepath.Rel(Includes, filepath.Dir(f))
			filename := strings.Replace(filepath.Base(f), ".scss", ".css", 1)
			fout = filepath.Join(BuildDir, rel, filename)
		} else {
			out = os.Stdout
		}

		ctx := context.Context{
			// TODO: Most of these fields are no longer used
			Sprites:     SpriteCache,
			Imgs:        ImageCache,
			OutputStyle: style,
			ImageDir:    Dir,
			FontDir:     Font,
			// Assumption that output is a file
			BuildDir:     filepath.Dir(fout),
			GenImgDir:    Gen,
			MainFile:     f,
			Comments:     Comments,
			IncludePaths: []string{filepath.Dir(f)},
		}
		if Includes != "" {
			ctx.IncludePaths = append(ctx.IncludePaths,
				strings.Split(Includes, ",")...)
		}
		fRead, err := os.Open(f)
		defer fRead.Close()
		if err != nil {
			log.Fatal(err)
		}
		if fout != "" {
			dir := filepath.Dir(fout)
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				log.Fatalf("Failed to create directory: %s", dir)
			}

			out, err = os.Create(fout)
			defer out.Close()
			if err != nil {
				log.Fatalf("Failed to create file: %s", f)
			}
			// log.Println("Created:", fout)
		}

		var pout bytes.Buffer
		par, err := startParser(&ctx, fRead, &pout, filepath.Dir(Input))
		if err != nil {
			log.Println(err)
			continue
		}
		err = ctx.Compile(&pout, out)

		if err != nil {
			n := ctx.ErrorLine()
			fs := par.LookupFile(n)
			log.Printf("Error encountered in: %s\n", fs)
			log.Println(err)
		}
	}
}

func startParser(ctx *context.Context, in io.Reader, out io.Writer, pkgdir string) (*sprite.Parser, error) {
	// Run the sprite_sass parser prior to passing to libsass
	parser := &sprite.Parser{
		ImageDir: ctx.ImageDir,
		Includes: ctx.IncludePaths,
		BuildDir: ctx.BuildDir,
		MainFile: ctx.MainFile,
	}
	// Save reference to parser in context
	bs, err := parser.Start(in, pkgdir)
	if err != nil {
		return parser, err
	}
	out.Write(bs)
	return parser, err
}
