package cursor

import (
	"encoding/binary"
	"fmt"
	"os"
)

// XCursorChunk represents a single chunk in the Xcursor file.
type XCursorChunk struct {
	Header XCursorImageHeader
	Pixels []byte
}

// XCursorImageHeader is the header for each image in the Xcursor file.
type XCursorImageHeader struct {
	Width    uint32
	Height   uint32
	XHot     uint32
	YHot     uint32
	Delay    uint32
}

// ParseXCursor reads an Xcursor binary file and returns all sprites found.
func ParseXCursor(path string) ([]*Sprite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 16 {
		return nil, fmt.Errorf("file too short")
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0x0000fe00 {
		return nil, fmt.Errorf("not a valid Xcursor file (magic=0x%08x)", magic)
	}

	// Header
	header := binary.LittleEndian.Uint32(data[4:8])
	_ = header // version or header size, not critical
	nImages := binary.LittleEndian.Uint32(data[8:12])
	_ = nImages

	var sprites []*Sprite
	offset := 12

	for offset+16 <= len(data) {
		// Read chunk header
		typeID := binary.LittleEndian.Uint32(data[offset : offset+4])
		subtype := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		length := binary.LittleEndian.Uint32(data[offset+8 : offset+12])
		_ = subtype

		if offset+12+int(length) > len(data) {
			break
		}

		if typeID == 0xFFFD0002 { // Image type
			imgData := data[offset+12 : offset+12+int(length)]
			if len(imgData) < 24 {
				offset += 12 + int(length)
				continue
			}

			w := binary.LittleEndian.Uint32(imgData[0:4])
			h := binary.LittleEndian.Uint32(imgData[4:8])
			xHot := binary.LittleEndian.Uint32(imgData[8:12])
			yHot := binary.LittleEndian.Uint32(imgData[12:16])
			// delay := binary.LittleEndian.Uint32(imgData[16:20]) // for animated cursors

			pixelData := imgData[20:]
			expectedLen := w * h * 4
			if uint32(len(pixelData)) < expectedLen {
				offset += 12 + int(length)
				continue
			}

			// Xcursor stores pixels as little-endian ARGB (0xAARRGGBB).
			// In memory that is [B, G, R, A]. OpenGL GL_RGBA expects [R, G, B, A].
			pixels := make([]byte, expectedLen)
			for i := 0; i < int(expectedLen); i += 4 {
				pixels[i] = pixelData[i+2]   // R
				pixels[i+1] = pixelData[i+1] // G
				pixels[i+2] = pixelData[i]   // B
				pixels[i+3] = pixelData[i+3] // A
			}

			s := &Sprite{
				Width:    int(w),
				Height:   int(h),
				HotspotX: int(xHot),
				HotspotY: int(yHot),
				Pixels:   pixels,
			}

			sprites = append(sprites, s)
		}

		offset += 12 + int(length)
	}

	return sprites, nil
}
