package autosetup

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	GGUFMagic = 0x46554747 // "GGUF" in little endian
)

// GGUF value types
const (
	GGUFTypeUInt8   = 0
	GGUFTypeInt8    = 1
	GGUFTypeUInt16  = 2
	GGUFTypeInt16   = 3
	GGUFTypeUInt32  = 4
	GGUFTypeInt32   = 5
	GGUFTypeFloat32 = 6
	GGUFTypeBool    = 7
	GGUFTypeString  = 8
	GGUFTypeArray   = 9
)

// GGUFMetadata contains the essential metadata for memory calculations
type GGUFMetadata struct {
	Architecture  string
	ModelName     string
	BlockCount    uint32
	ContextLength uint32
	HeadCountKV   uint32
	KeyLength     uint32
	ValueLength   uint32
	SlidingWindow uint32
}

// GGUFReader reads GGUF file metadata
type GGUFReader struct {
	file     *os.File
	metadata *GGUFMetadata
}

// NewGGUFReader creates a new GGUF reader
func NewGGUFReader(filepath string) (*GGUFReader, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return &GGUFReader{
		file:     file,
		metadata: &GGUFMetadata{},
	}, nil
}

// Close closes the file
func (r *GGUFReader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// ReadMetadata reads and parses the GGUF metadata
func (r *GGUFReader) ReadMetadata() (*GGUFMetadata, error) {
	// Read GGUF header
	var magic, version uint32
	var tensorCount, metadataKVCount uint64

	if err := binary.Read(r.file, binary.LittleEndian, &magic); err != nil {
		return nil, fmt.Errorf("failed to read magic: %w", err)
	}

	if magic != GGUFMagic {
		return nil, fmt.Errorf("invalid GGUF magic number: 0x%x", magic)
	}

	if err := binary.Read(r.file, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}

	if err := binary.Read(r.file, binary.LittleEndian, &tensorCount); err != nil {
		return nil, fmt.Errorf("failed to read tensor count: %w", err)
	}

	if err := binary.Read(r.file, binary.LittleEndian, &metadataKVCount); err != nil {
		return nil, fmt.Errorf("failed to read metadata KV count: %w", err)
	}

	// Read metadata key-value pairs
	if err := r.readMetadataKVs(metadataKVCount); err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	return r.metadata, nil
}

// readMetadataKVs reads the metadata key-value pairs
func (r *GGUFReader) readMetadataKVs(count uint64) error {
	keysToRead := map[string]bool{
		"general.architecture": true,
		"general.name":         true,
	}

	archSpecificKeysAdded := false

	for i := uint64(0); i < count; i++ {
		// Read key
		key, err := r.readString()
		if err != nil {
			return fmt.Errorf("failed to read key %d: %w", i, err)
		}

		// Read value type
		var valueType uint32
		if err := binary.Read(r.file, binary.LittleEndian, &valueType); err != nil {
			return fmt.Errorf("failed to read value type for key %s: %w", key, err)
		}

		// Add architecture-specific keys once we know the architecture
		if !archSpecificKeysAdded && r.metadata.Architecture != "" {
			prefix := r.metadata.Architecture
			keysToRead[prefix+".block_count"] = true
			keysToRead[prefix+".context_length"] = true
			keysToRead[prefix+".attention.head_count_kv"] = true
			keysToRead[prefix+".attention.key_length"] = true
			keysToRead[prefix+".attention.value_length"] = true
			keysToRead[prefix+".attention.sliding_window_size"] = true

			// Additional sliding window keys that some models might use
			keysToRead[prefix+".attention.sliding_window"] = true
			keysToRead["general.sliding_window_size"] = true
			keysToRead["attention.sliding_window_size"] = true
			keysToRead["sliding_window_size"] = true

			archSpecificKeysAdded = true
		}

		// Read value if it's a key we care about
		if keysToRead[key] {
			if err := r.readAndStoreValue(key, valueType); err != nil {
				return fmt.Errorf("failed to read value for key %s: %w", key, err)
			}
		} else {
			// Skip value
			if err := r.skipValue(valueType); err != nil {
				return fmt.Errorf("failed to skip value for key %s: %w", key, err)
			}
		}
	}

	return nil
}

// readString reads a GGUF string
func (r *GGUFReader) readString() (string, error) {
	var length uint64
	if err := binary.Read(r.file, binary.LittleEndian, &length); err != nil {
		return "", err
	}

	if length == 0 {
		return "", nil
	}

	bytes := make([]byte, length)
	if _, err := io.ReadFull(r.file, bytes); err != nil {
		return "", err
	}

	return string(bytes), nil
}

// readAndStoreValue reads a value and stores it in metadata
func (r *GGUFReader) readAndStoreValue(key string, valueType uint32) error {
	switch key {
	case "general.architecture":
		if valueType != GGUFTypeString {
			return r.skipValue(valueType)
		}
		arch, err := r.readString()
		if err != nil {
			return err
		}
		r.metadata.Architecture = arch

	case "general.name":
		if valueType != GGUFTypeString {
			return r.skipValue(valueType)
		}
		name, err := r.readString()
		if err != nil {
			return err
		}
		r.metadata.ModelName = name

	default:
		// Architecture-specific keys
		if strings.HasSuffix(key, ".block_count") {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.BlockCount = value
			} else {
				return r.skipValue(valueType)
			}

		} else if strings.HasSuffix(key, ".context_length") {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.ContextLength = value
			} else {
				return r.skipValue(valueType)
			}

		} else if strings.HasSuffix(key, ".attention.head_count_kv") {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.HeadCountKV = value
			} else {
				return r.skipValue(valueType)
			}

		} else if strings.HasSuffix(key, ".attention.key_length") {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.KeyLength = value
			} else {
				return r.skipValue(valueType)
			}

		} else if strings.HasSuffix(key, ".attention.value_length") {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.ValueLength = value
			} else {
				return r.skipValue(valueType)
			}

		} else if strings.HasSuffix(key, ".attention.sliding_window_size") {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.SlidingWindow = value
			} else {
				return r.skipValue(valueType)
			}

		} else if strings.HasSuffix(key, ".attention.sliding_window") {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.SlidingWindow = value
			} else {
				return r.skipValue(valueType)
			}

		} else if key == "general.sliding_window_size" || key == "attention.sliding_window_size" || key == "sliding_window_size" {
			if valueType == GGUFTypeUInt32 {
				var value uint32
				if err := binary.Read(r.file, binary.LittleEndian, &value); err != nil {
					return err
				}
				r.metadata.SlidingWindow = value
			} else {
				return r.skipValue(valueType)
			}

		} else {
			return r.skipValue(valueType)
		}
	}

	return nil
}

// skipValue skips over a value of the given type
func (r *GGUFReader) skipValue(valueType uint32) error {
	switch valueType {
	case GGUFTypeUInt8, GGUFTypeInt8, GGUFTypeBool:
		_, err := r.file.Seek(1, io.SeekCurrent)
		return err
	case GGUFTypeUInt16, GGUFTypeInt16:
		_, err := r.file.Seek(2, io.SeekCurrent)
		return err
	case GGUFTypeUInt32, GGUFTypeInt32, GGUFTypeFloat32:
		_, err := r.file.Seek(4, io.SeekCurrent)
		return err
	case GGUFTypeString:
		// Read length and skip string data
		var length uint64
		if err := binary.Read(r.file, binary.LittleEndian, &length); err != nil {
			return err
		}
		_, err := r.file.Seek(int64(length), io.SeekCurrent)
		return err
	case GGUFTypeArray:
		// Read array type and count
		var arrayType uint32
		var count uint64
		if err := binary.Read(r.file, binary.LittleEndian, &arrayType); err != nil {
			return err
		}
		if err := binary.Read(r.file, binary.LittleEndian, &count); err != nil {
			return err
		}

		// Skip array elements
		switch arrayType {
		case GGUFTypeUInt8, GGUFTypeInt8, GGUFTypeBool:
			_, err := r.file.Seek(int64(count), io.SeekCurrent)
			return err
		case GGUFTypeUInt16, GGUFTypeInt16:
			_, err := r.file.Seek(int64(count*2), io.SeekCurrent)
			return err
		case GGUFTypeUInt32, GGUFTypeInt32, GGUFTypeFloat32:
			_, err := r.file.Seek(int64(count*4), io.SeekCurrent)
			return err
		case GGUFTypeString:
			// Skip each string individually
			for i := uint64(0); i < count; i++ {
				if err := r.skipValue(GGUFTypeString); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("unsupported array type for skipping: %d", arrayType)
		}
	default:
		return fmt.Errorf("unsupported value type for skipping: %d", valueType)
	}
}

// ReadGGUFMetadata is a convenience function to read metadata from a GGUF file
func ReadGGUFMetadata(filepath string) (*GGUFMetadata, error) {
	reader, err := NewGGUFReader(filepath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return reader.ReadMetadata()
}

// ReadAllGGUFKeys reads all metadata keys from a GGUF file (for debugging mmproj files)
func ReadAllGGUFKeys(filepath string) (map[string]interface{}, error) {
	reader, err := NewGGUFReader(filepath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Read GGUF header
	var magic, version uint32
	var tensorCount, metadataKVCount uint64

	if err := binary.Read(reader.file, binary.LittleEndian, &magic); err != nil {
		return nil, fmt.Errorf("failed to read magic: %w", err)
	}

	if magic != GGUFMagic {
		return nil, fmt.Errorf("invalid GGUF magic number: 0x%x", magic)
	}

	if err := binary.Read(reader.file, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}

	if err := binary.Read(reader.file, binary.LittleEndian, &tensorCount); err != nil {
		return nil, fmt.Errorf("failed to read tensor count: %w", err)
	}

	if err := binary.Read(reader.file, binary.LittleEndian, &metadataKVCount); err != nil {
		return nil, fmt.Errorf("failed to read metadata KV count: %w", err)
	}

	// Read ALL metadata key-value pairs
	allKeys := make(map[string]interface{})

	for i := uint64(0); i < metadataKVCount; i++ {
		// Read key
		key, err := reader.readString()
		if err != nil {
			return nil, fmt.Errorf("failed to read key %d: %w", i, err)
		}

		// Read value type
		var valueType uint32
		if err := binary.Read(reader.file, binary.LittleEndian, &valueType); err != nil {
			return nil, fmt.Errorf("failed to read value type for key %s: %w", key, err)
		}

		// Read value based on type
		value, err := reader.readAnyValue(valueType)
		if err != nil {
			return nil, fmt.Errorf("failed to read value for key %s: %w", key, err)
		}

		allKeys[key] = value
	}

	return allKeys, nil
}

// readAnyValue reads a value of any supported GGUF type
func (r *GGUFReader) readAnyValue(valueType uint32) (interface{}, error) {
	switch valueType {
	case GGUFTypeUInt8:
		var value uint8
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value, err
	case GGUFTypeInt8:
		var value int8
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value, err
	case GGUFTypeUInt16:
		var value uint16
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value, err
	case GGUFTypeInt16:
		var value int16
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value, err
	case GGUFTypeUInt32:
		var value uint32
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value, err
	case GGUFTypeInt32:
		var value int32
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value, err
	case GGUFTypeFloat32:
		var value float32
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value, err
	case GGUFTypeBool:
		var value uint8
		err := binary.Read(r.file, binary.LittleEndian, &value)
		return value != 0, err
	case GGUFTypeString:
		return r.readString()
	case GGUFTypeArray:
		// For arrays, just skip for now (complex to parse)
		err := r.skipValue(valueType)
		return nil, err
	default:
		err := r.skipValue(valueType)
		return nil, err
	}
}
