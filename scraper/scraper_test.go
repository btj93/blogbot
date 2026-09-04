package scraper

import (
	"testing"
)

func TestBatchImagesGrouped(t *testing.T) {
	imgs := make([][]byte, 25)
	for i := range imgs {
		imgs[i] = []byte{byte(i)}
	}

	batches := BatchImagesGrouped(imgs, 10)
	if len(batches) != 3 {
		t.Errorf("got %d batches, want 3", len(batches))
	}

	if len(batches[0]) != 10 {
		t.Errorf("batch 0 size=%d, want 10", len(batches[0]))
	}

	if len(batches[2]) != 5 {
		t.Errorf("batch 2 size=%d, want 5", len(batches[2]))
	}
}

func TestBatchImagesGroupedEmpty(t *testing.T) {
	batches := BatchImagesGrouped(nil, 10)
	if batches != nil {
		t.Errorf("got %v, want nil", batches)
	}
}

func TestBatchImagesGrouped_Exactly10(t *testing.T) {
	imgs := make([][]byte, 10)
	for i := range imgs {
		imgs[i] = []byte{byte(i)}
	}

	batches := BatchImagesGrouped(imgs, 10)
	if len(batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(batches))
	}

	if len(batches[0]) != 10 {
		t.Errorf("batch 0 size=%d, want 10", len(batches[0]))
	}
}

func TestBatchImagesGrouped_11(t *testing.T) {
	imgs := make([][]byte, 11)
	for i := range imgs {
		imgs[i] = []byte{byte(i)}
	}

	batches := BatchImagesGrouped(imgs, 10)
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(batches))
	}

	if len(batches[0]) != 10 {
		t.Errorf("batch 0 size=%d, want 10", len(batches[0]))
	}

	if len(batches[1]) != 1 {
		t.Errorf("batch 1 size=%d, want 1", len(batches[1]))
	}
}

func TestBatchImagesGrouped_Zero(t *testing.T) {
	batches := BatchImagesGrouped(nil, 10)
	if batches != nil {
		t.Errorf("got %v, want nil", batches)
	}
}
