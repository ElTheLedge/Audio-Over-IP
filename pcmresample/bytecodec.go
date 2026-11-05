package pcmresample

import (
	"encoding/binary"
	"errors"
	"math"
)

const q31 = 1 << 31

// decodeBytesToPlanar converts interleaved []byte PCM to planar [][]float32 normalized to [-1,1).
func decodeBytesToPlanar(in []byte, fmt SampleFormat, channels int) ([][]float32, error) {
	if len(in)%(channels*4) != 0 {
		return nil, errors.New("input length not aligned to frame size")
	}
	frames := len(in) / (channels * 4)
	out := make([][]float32, channels)
	for c := range out {
		out[c] = make([]float32, frames)
	}
	// Interleaved little-endian
	for i := 0; i < frames; i++ {
		base := i * channels * 4
		for c := 0; c < channels; c++ {
			off := base + c*4
			u := binary.LittleEndian.Uint32(in[off : off+4])
			switch fmt {
			case PCM32Float:
				f := math.Float32frombits(u)
				out[c][i] = f
			case PCM32Int:
				val := int32(u)
				// Normalize signed Q31 to [-1, 1)
				out[c][i] = float32(float64(val) * (1.0 / q31))
			default:
				return nil, errors.New("unsupported format")
			}
		}
	}
	return out, nil
}

// encodePlanarToBytes converts planar [][]float32 into interleaved []byte in requested format.
func encodePlanarToBytes(in [][]float32, fmt SampleFormat) ([]byte, error) {
	if len(in) == 0 {
		return []byte{}, nil
	}
	channels := len(in)
	frames := len(in[0])
	for c := 1; c < channels; c++ {
		if len(in[c]) != frames {
			return nil, errors.New("planar channel length mismatch")
		}
	}
	out := make([]byte, frames*channels*4)
	for i := 0; i < frames; i++ {
		base := i * channels * 4
		for c := 0; c < channels; c++ {
			v := in[c][i]
			off := base + c*4
			switch fmt {
			case PCM32Float:
				binary.LittleEndian.PutUint32(out[off:off+4], math.Float32bits(v))
			case PCM32Int:
				// Clip to [-1, 1) then convert to int32
				if v >= 1.0 {
					v = math.Nextafter32(1.0, 0.0) // use float32-safe nextafter
				}
				if v < -1.0 {
					v = -1.0
				}
				scaled := v * q31
				s := int32(scaled)
				if s == math.MinInt32 {
					s = math.MinInt32 + 1
				}

				binary.LittleEndian.PutUint32(out[off:off+4], uint32(s))
			default:
				return nil, errors.New("unsupported format")
			}
		}
	}
	return out, nil
}
