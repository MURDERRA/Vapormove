package cursor

/*
#include <stdlib.h>
#include <GL/gl.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// GLTexture wraps an OpenGL texture ID for a sprite.
type GLTexture struct {
	ID     uint32
	Width  int32
	Height int32
}

// UploadSprite loads a Sprite into an OpenGL texture.
// Returns the texture ID or 0 on failure.
func UploadSprite(s *Sprite) uint32 {
	if s == nil || s.Width == 0 || s.Height == 0 {
		return 0
	}

	var texID C.GLuint
	C.glGenTextures(1, &texID)
	if texID == 0 {
		return 0
	}

	C.glBindTexture(C.GL_TEXTURE_2D, texID)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)

	C.glTexImage2D(C.GL_TEXTURE_2D, 0, C.GL_RGBA,
		C.GLsizei(s.Width), C.GLsizei(s.Height), 0,
		C.GL_RGBA, C.GL_UNSIGNED_BYTE,
		 unsafe.Pointer(&s.Pixels[0]))

	return uint32(texID)
}

// DeleteTexture frees an OpenGL texture.
func DeleteTexture(id uint32) {
	if id != 0 {
		C.glDeleteTextures(1, (*C.GLuint)(unsafe.Pointer(&id)))
	}
}

// TextureManager caches and manages OpenGL textures for sprite rendering.
type TextureManager struct {
	currentSprite *Sprite
	texID         uint32
	valid         bool
}

// NewTextureManager creates a new texture manager.
func NewTextureManager() *TextureManager {
	return &TextureManager{}
}

// LoadSprite uploads a sprite as a texture, reusing the existing ID if possible.
func (tm *TextureManager) LoadSprite(s *Sprite) error {
	if s == nil {
		return fmt.Errorf("nil sprite")
	}
	if s == tm.currentSprite && tm.valid {
		return nil // already loaded
	}

	if tm.valid {
		DeleteTexture(tm.texID)
	}

	tm.texID = UploadSprite(s)
	if tm.texID == 0 {
		return fmt.Errorf("failed to upload sprite texture")
	}

	tm.currentSprite = s
	tm.valid = true
	return nil
}

// Bind selects this texture for drawing via OpenGL.
func (tm *TextureManager) Bind() {
	if tm.valid {
		C.glBindTexture(C.GL_TEXTURE_2D, C.GLuint(tm.texID))
	} else {
		C.glBindTexture(C.GL_TEXTURE_2D, 0)
	}
}

// Release frees the OpenGL texture.
func (tm *TextureManager) Release() {
	if tm.valid {
		DeleteTexture(tm.texID)
		tm.valid = false
		tm.currentSprite = nil
	}
}
