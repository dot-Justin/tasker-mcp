package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedToolDescriptionsMatchDist(t *testing.T) {
	embeddedPath := filepath.Join("assets", "toolDescriptions.json")
	embeddedData, err := os.ReadFile(embeddedPath)
	if err != nil {
		t.Fatalf("failed to read embedded asset %s: %v", embeddedPath, err)
	}

	distPath := filepath.Join("..", "dist", "toolDescriptions.json")
	distData, err := os.ReadFile(distPath)
	if err != nil {
		t.Fatalf("failed to read dist asset %s: %v", distPath, err)
	}

	if !bytes.Equal(embeddedData, distData) {
		t.Fatalf("embedded tool descriptions do not match %s", distPath)
	}
}
