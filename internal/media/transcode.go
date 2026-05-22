// Package media holds small server-side helpers that adapt user-uploaded
// media so it can be forwarded to upstream APIs (Meta's WhatsApp Cloud
// API in particular) that only accept specific container formats.
package media

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// NormalizeForWhatsApp inspects the incoming media MIME type and, when it is
// a container Meta refuses (audio/webm in particular — recorded by Chrome's
// default MediaRecorder), uses ffmpeg to repackage the stream into a
// container Meta accepts (audio/ogg with Opus). The audio codec is copied,
// not re-encoded, so the operation is cheap and lossless.
//
// Returns the (possibly rewritten) data, the new MIME type, and the
// extension to use for the filename. If no conversion is needed, the input
// is returned untouched.
//
// On any ffmpeg failure the original bytes are returned so the caller can
// still attempt the upload — Meta's own error will surface to the user.
func NormalizeForWhatsApp(ctx context.Context, data []byte, mimeType string) ([]byte, string, string) {
	lower := strings.ToLower(mimeType)
	if !strings.HasPrefix(lower, "audio/webm") {
		return data, mimeType, extensionForMime(mimeType)
	}

	out, err := remuxWebmToOgg(ctx, data)
	if err != nil {
		// Caller logs; here we just fall back to the original payload.
		return data, mimeType, extensionForMime(mimeType)
	}
	return out, "audio/ogg", ".ogg"
}

// remuxWebmToOgg copies the Opus audio stream from a webm container into
// an ogg container. Browser MediaRecorder produces webm/opus by default;
// Meta wants ogg/opus, and the codec is already compatible so we can
// stream-copy without re-encoding.
func remuxWebmToOgg(parent context.Context, data []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()

	// -f webm forces the demuxer (Chrome sometimes omits the magic bytes
	// after a cut), -c:a copy keeps the Opus stream as-is.
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "webm", "-i", "pipe:0",
		"-c:a", "copy",
		"-f", "ogg", "pipe:1",
	)
	cmd.Stdin = bytes.NewReader(data)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg remux failed: %w (stderr: %s)", err, stderr.String())
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced empty output: %s", stderr.String())
	}
	return stdout.Bytes(), nil
}

// extensionForMime returns the conventional file extension for a MIME type,
// or empty if unknown.
func extensionForMime(mime string) string {
	switch strings.ToLower(mime) {
	case "audio/ogg", "audio/opus":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4", "audio/aac":
		return ".m4a"
	case "audio/amr":
		return ".amr"
	case "audio/webm":
		return ".webm"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/3gpp":
		return ".3gp"
	case "application/pdf":
		return ".pdf"
	}
	return ""
}
