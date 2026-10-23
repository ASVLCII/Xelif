package filepreview

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/yorukot/superfile/src/internal/common"
)

type thumbnailGeneratorInterface interface {
	supportsExt(ext string) bool
	generateThumbnail(inputPath string, outputPathWithoutExt string) (string, error)
}

type VideoGenerator struct{}

func newVideoGenerator() (*VideoGenerator, error) {
	if !isFFmpegInstalled() {
		return nil, errors.New("ffmpeg is not installed")
	}

	return &VideoGenerator{}, nil
}

func (g *VideoGenerator) supportsExt(ext string) bool {
	return common.VideoExtensions[strings.ToLower(ext)]
}

func (g *VideoGenerator) generateThumbnail(inputPath string, outputPathWithoutExt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), thumbGenerationTimeout)
	defer cancel()
	outputPath := outputPathWithoutExt + thumbOutputExt

	// ffmpeg -v warning -t 60 -hwaccel auto -an -sn -dn -skip_frame nokey -i input.mkv -vf scale='min(1024,iw)':'min(720,ih)':force_original_aspect_ratio=decrease:flags=fast_bilinear -vf "thumbnail" -frames:v 1 -y thumb.jpg
	ffmpeg := exec.CommandContext(ctx, "ffmpeg",
		"-v", "warning", // set log level to warning
		"-an",       // disable Audio stream
		"-sn",       // disable Subtitle stream
		"-dn",       // disable data stream
		"-t", "180", // process maximum 180s of the video (the first 3 min)
		"-hwaccel", "auto", // Use Hardware Acceleration if available
		"-skip_frame", "nokey", // skip non-key frames
		"-i", inputPath, // set input file
		"-vf", "thumbnail", // use ffmpeg default thumbnail filter
		"-frames:v", "1", // output only one frame (one image)
		"-f", "image2", // set format to image2
		"-fs", maxVideoFileSizeForThumb, // limit the max file size to match image previewer limit
		"-y", outputPath, // set the outputFile and overwrite it without confirmation if already exists
	)

	err := ffmpeg.Run()
	if err != nil {
		return "", fmt.Errorf("error generating video thumbnail, outputPath: %s : %w", outputPath, err)
	}

	return outputPath, nil
}

type pdfGenerator struct{}

func newPdfGenerator() (*pdfGenerator, error) {
	if !isPopplerInstalled() {
		return nil, errors.New("poppler is not installed")
	}

	return &pdfGenerator{}, nil
}

func (g *pdfGenerator) supportsExt(ext string) bool {
	return strings.ToLower(ext) == ".pdf"
}

func (g *pdfGenerator) generateThumbnail(inputPath string, outputPathWithoutExt string) (string, error) {
	outputPath := outputPathWithoutExt + thumbOutputExt
	ctx, cancel := context.WithTimeout(context.Background(), thumbGenerationTimeout)
	defer cancel()

	// pdftoppm -singlefile -png prefixFilename
	pdftoppm := exec.CommandContext(ctx, "pdftoppm",
		"-singlefile",        // output only the first page as image
		"-jpeg",              // Image extension
		inputPath,            // Set input file
		outputPathWithoutExt, // The output prefix. (pdftoppm will add the .jpg ext)
	)

	err := pdftoppm.Run()
	if err != nil {
		return "", fmt.Errorf("error generating pdf thumbnail, outputPath: %s : %w",
			outputPath, err)
	}

	return outputPath, nil
}

type psGenerator struct{}

func newPsGenerator() (*psGenerator, error) {
	if !isGhostscriptInstalled() {
		return nil, errors.New("ghostscript is not installed")
	}

	return &psGenerator{}, nil
}

func (g *psGenerator) supportsExt(ext string) bool {
	extension := strings.ToLower(ext)
	return extension == ".ps" || extension == ".eps"
}

func (g *psGenerator) generateThumbnail(inputPath string, outputPathWithoutExt string) (string, error) {
	outputPath := outputPathWithoutExt + thumbOutputExt
	ctx, cancel := context.WithTimeout(context.Background(), thumbGenerationTimeout)
	defer cancel()

	// gs -dSAFER -dBATCH -dNOPAUSE -sPageList=1 -sDEVICE=jpeg -r150 -sOutputFile=output.jpg input.ps
	outputParam := "-sOutputFile=" + outputPath
	gs := exec.CommandContext(ctx, "gs",
		"-dSAFER", "-dBATCH", "-dNOPAUSE", // Standard GS operators
		"-sPageList=1",  // Output only the first page
		"-sDEVICE=jpeg", // Output format
		"-r150",         // Resolution (the same as for pdf)
		outputParam,     // Result (variable because of golangci-lint)
		inputPath,       // Input file
	)

	err := gs.Run()
	if err != nil {
		return "", fmt.Errorf("error generating ps thumbnail, outputPath: %s : %w",
			outputPath, err)
	}

	return outputPath, nil
}

type ThumbnailGenerator struct {
	// This is a cache. Key -> Video file path, Value -> Thumbnail file path
	// TODO: We can potentially make it persistent, preventing generation
	// of thumbnail on every launch or superfile
	tempFilesCache map[string]thumbnailCacheEntry
	tempDirectory  string
	mu             sync.Mutex
	generators     []thumbnailGeneratorInterface
	inFlight       singleflight.Group
}

type thumbnailCacheEntry struct {
	path       string
	sourceSize int64
	sourceTime time.Time
}

func NewThumbnailGenerator() (*ThumbnailGenerator, error) {
	tmp, err := os.MkdirTemp("", "superfiles-*")
	if err != nil {
		return nil, err
	}

	generators := []thumbnailGeneratorInterface{}

	pdf, err := newPdfGenerator()
	if err != nil {
		slog.Debug("Error while trying to create pdfGenerator", "error", err)
	} else {
		generators = append(generators, pdf)
	}

	ps, err := newPsGenerator()
	if err != nil {
		slog.Debug("Error while trying to create psGenerator", "error", err)
	} else {
		generators = append(generators, ps)
	}

	video, err := newVideoGenerator()
	if err != nil {
		slog.Debug("Error while trying to create videoGenerator", "error", err)
	} else {
		generators = append(generators, video)
	}

	thumbnailGenerator := &ThumbnailGenerator{
		tempFilesCache: make(map[string]thumbnailCacheEntry),
		tempDirectory:  tmp,
		generators:     generators,
	}

	return thumbnailGenerator, nil
}

func (g *ThumbnailGenerator) SupportsExt(ext string) bool {
	for i := range g.generators {
		if g.generators[i].supportsExt(ext) {
			return true
		}
	}

	return false
}

func (g *ThumbnailGenerator) GetThumbnailOrGenerate(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	// Navigation and resizing may overlap requests. Share the expensive external
	// conversion only when both the path and observed source revision match.
	key := fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano())
	result, err, _ := g.inFlight.Do(key, func() (any, error) {
		return g.thumbnailForSource(path, info)
	})
	if err != nil {
		return "", err
	}
	thumbnail, ok := result.(string)
	if !ok {
		return "", errors.New("invalid thumbnail conversion result")
	}
	return thumbnail, nil
}

func (g *ThumbnailGenerator) thumbnailForSource(path string, info os.FileInfo) (string, error) {
	g.mu.Lock()
	file, ok := g.tempFilesCache[path]
	g.mu.Unlock()

	if ok && file.sourceSize == info.Size() && file.sourceTime.Equal(info.ModTime()) {
		_, err := os.Stat(file.path)
		if err == nil {
			return file.path, nil
		}
	}

	generatedThumbnailPath, err := g.generateThumbnail(path)
	if err != nil {
		return "", err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	current, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		// A newer revision may already have completed while this conversion was
		// running. Return this request's result without replacing the new cache.
		return generatedThumbnailPath, nil
	}

	g.tempFilesCache[path] = thumbnailCacheEntry{
		path: generatedThumbnailPath, sourceSize: info.Size(), sourceTime: info.ModTime(),
	}
	// Retain old paths until CleanUp: concurrent preview commands may still be
	// opening an earlier result after GetThumbnailOrGenerate has returned it.

	return generatedThumbnailPath, nil
}

func (g *ThumbnailGenerator) generateThumbnail(path string) (string, error) {
	fileExt := filepath.Ext(path)
	for index := range g.generators {
		generator := g.generators[index]

		if !generator.supportsExt(fileExt) {
			continue
		}
		filename := filepath.Base(path)
		baseName := filename[:len(filename)-len(fileExt)]

		outputPathWithoutExt := filepath.Join(g.tempDirectory,
			fmt.Sprintf("%s-%d", baseName, time.Now().UnixNano()))

		outputPath, err := generator.generateThumbnail(path, outputPathWithoutExt)
		if err != nil {
			return "", err
		}

		return outputPath, nil
	}

	return "", errors.New("unsupported file format")
}

func (g *ThumbnailGenerator) CleanUp() error {
	return os.RemoveAll(g.tempDirectory)
}

func isPopplerInstalled() bool {
	_, err := exec.LookPath("pdftoppm")
	return err == nil
}

func isGhostscriptInstalled() bool {
	_, err := exec.LookPath("gs")
	return err == nil
}

func isFFmpegInstalled() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
