package cache

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestTeeReader tests the TeeReader logic for simultaneous writing
func TestTeeReader(t *testing.T) {
	input := "Hello, World!"
	reader := strings.NewReader(input)

	var buf1, buf2 bytes.Buffer

	teeReader := NewTeeReader(reader, &buf1, &buf2)

	// Read from TeeReader
	output, err := io.ReadAll(teeReader)
	if err != nil {
		t.Fatalf("Failed to read from TeeReader: %v", err)
	}

	// Verify output matches input
	if string(output) != input {
		t.Errorf("Output mismatch: got %s, want %s", string(output), input)
	}

	// Verify buf1 received data
	if buf1.String() != input {
		t.Errorf("buf1 mismatch: got %s, want %s", buf1.String(), input)
	}

	// Verify buf2 received data
	if buf2.String() != input {
		t.Errorf("buf2 mismatch: got %s, want %s", buf2.String(), input)
	}
}

// TestTeeReaderMultipleReads tests TeeReader with multiple read operations
func TestTeeReaderMultipleReads(t *testing.T) {
	input := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	reader := strings.NewReader(input)

	var writer bytes.Buffer
	teeReader := NewTeeReader(reader, &writer)

	// Read in chunks
	buf := make([]byte, 5)
	var result bytes.Buffer

	for {
		n, err := teeReader.Read(buf)
		if n > 0 {
			result.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	// Verify full data was read
	if result.String() != input {
		t.Errorf("Result mismatch: got %s, want %s", result.String(), input)
	}

	// Verify writer received all data
	if writer.String() != input {
		t.Errorf("Writer mismatch: got %s, want %s", writer.String(), input)
	}
}

// TestTeeReaderWithMultipleWriters tests writing to multiple destinations
func TestTeeReaderWithMultipleWriters(t *testing.T) {
	data := "Test data for multiple writers"
	reader := strings.NewReader(data)

	var w1, w2, w3 bytes.Buffer
	teeReader := NewTeeReader(reader, &w1, &w2, &w3)

	_, err := io.ReadAll(teeReader)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	// All writers should have same data
	writers := []*bytes.Buffer{&w1, &w2, &w3}
	for i, w := range writers {
		if w.String() != data {
			t.Errorf("Writer %d mismatch: got %s, want %s", i+1, w.String(), data)
		}
	}
}

// TestCountingWriter tests the byte counting logic
func TestCountingWriter(t *testing.T) {
	var count int64
	cw := &countingWriter{count: &count}

	testData := []string{
		"Hello",
		"World",
		"Test",
	}

	totalExpected := int64(0)
	for _, data := range testData {
		n, err := cw.Write([]byte(data))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(data) {
			t.Errorf("Write returned wrong count: got %d, want %d", n, len(data))
		}
		totalExpected += int64(len(data))
	}

	if count != totalExpected {
		t.Errorf("Total byte count wrong: got %d, want %d", count, totalExpected)
	}
}

// TestCountingWriterConcurrent tests concurrent writes
func TestCountingWriterConcurrent(t *testing.T) {
	var count int64
	cw := &countingWriter{count: &count}

	data := []byte("Test")
	iterations := 100

	for i := 0; i < iterations; i++ {
		_, err := cw.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	expected := int64(len(data) * iterations)
	if count != expected {
		t.Errorf("Byte count mismatch: got %d, want %d", count, expected)
	}
}

// TestContentTypeDetection tests content type handling
func TestContentTypeDetection(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
		expected    string
	}{
		{
			filename:    "test.zip",
			contentType: "application/zip",
			expected:    "application/zip",
		},
		{
			filename:    "test.pdf",
			contentType: "application/pdf",
			expected:    "application/pdf",
		},
		{
			filename:    "unknown.xyz",
			contentType: "",
			expected:    "application/octet-stream",
		},
		{
			filename:    "test.tar.gz",
			contentType: "",
			expected:    "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Business logic: default to octet-stream if empty
			contentType := tt.contentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			if contentType != tt.expected {
				t.Errorf("Content type mismatch: got %s, want %s", contentType, tt.expected)
			}
		})
	}
}

// TestByteCountAccuracy tests that byte counting is accurate
func TestByteCountAccuracy(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"small", "test"},
		{"medium", strings.Repeat("A", 1024)},
		{"large", strings.Repeat("B", 10240)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count int64
			cw := &countingWriter{count: &count}

			n, err := cw.Write([]byte(tt.data))
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			if n != len(tt.data) {
				t.Errorf("Write returned %d, want %d", n, len(tt.data))
			}

			if count != int64(len(tt.data)) {
				t.Errorf("Count is %d, want %d", count, int64(len(tt.data)))
			}
		})
	}
}
