package genai

import (
	"bytes"
	"testing"

	"google.golang.org/genai"
)

func TestExtractFinalImageBytes_SkipsThoughtImage(t *testing.T) {
	thoughtDraft := []byte("thought-draft-1k")
	finalRender := []byte("final-render-2k")

	parts := []*genai.Part{
		{
			Thought: true,
			Text:    "Thinking about composition...",
		},
		{
			Thought: true,
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     thoughtDraft,
			},
		},
		{
			Thought: false,
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     finalRender,
			},
		},
	}

	got := extractFinalImageBytes(parts)
	if !bytes.Equal(got, finalRender) {
		t.Fatalf("extractFinalImageBytes(thought+final) = %q, want %q", string(got), string(finalRender))
	}
}

func TestExtractFinalImageBytes_FallbackWhenOnlyThoughtPresent(t *testing.T) {
	thoughtDraft := []byte("only-thought-image")
	parts := []*genai.Part{
		{
			Thought: true,
			InlineData: &genai.Blob{
				MIMEType: "image/png",
				Data:     thoughtDraft,
			},
		},
	}

	got := extractFinalImageBytes(parts)
	if !bytes.Equal(got, thoughtDraft) {
		t.Fatalf("extractFinalImageBytes(thoughtOnly) = %q, want %q", string(got), string(thoughtDraft))
	}
}

func TestExtractGeneratedVideoURI(t *testing.T) {
	v := &genai.GeneratedVideo{
		Video: &genai.Video{
			URI: "gs://my-bucket/videos/out.mp4",
		},
	}
	if got := extractGeneratedVideoURI(v); got != "gs://my-bucket/videos/out.mp4" {
		t.Fatalf("extractGeneratedVideoURI(v) = %q, want %q", got, "gs://my-bucket/videos/out.mp4")
	}
}
