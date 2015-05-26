package testing1

import (
	cryptoRand "crypto/rand"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

const (
	packetIterations = 10000

	uniformChunkLen   = 4096
	uniformChunkCount = 10
)

type Encoder interface {
	MaxPacketLength() uint16
	NextPacketLength() uint16
	NextPacketSleep() time.Duration
	ShapeBytes(dst, src []byte) (dn, sn int)
}

type Decoder interface {
	UnshapeBytes(dst, src []byte) (dn, sn int)
}

func TestExpectedPerformance(t *testing.T, enc Encoder) {
	max := enc.MaxPacketLength()
	var totalBytes uint64
	var totalDuration time.Duration

	for i := 0; i < packetIterations; i++ {
		send := enc.NextPacketLength()
		switch {
		case 0 == send:
			t.Fatalf("encoder wanted to send zero-length packet")
		case max < send:
			t.Fatalf("encoder wanted to send len %d > %d", send, max)
		}

		sleep := enc.NextPacketSleep()
		if sleep < time.Duration(0) {
			t.Fatalf("encoder wanted to sleep negative amount")
		}

		totalBytes += uint64(send)
		totalDuration += sleep
	}

	totalSeconds := totalDuration.Seconds()
	bytesPerSecond := float64(totalBytes) / totalSeconds
	bitsPerSecond := 8.0 * bytesPerSecond
	t.Logf("simulated average shaped transfer rate: %0.2e B/s = %0.2e b/s (%d B / %0.2f s; granularity %0.3f s)", bytesPerSecond, bitsPerSecond, totalBytes, totalSeconds, totalSeconds / float64(packetIterations))

	uniformSource := make([]byte, uniformChunkLen)
	_, err := cryptoRand.Read(uniformSource)
	if err != nil {
		t.Fatalf("cannot get random bytes: %v", err)
	}

	var shapedDist [256]uint64

	totalSource := uniformChunkLen * uniformChunkCount
	var sourceConsumed int
	var shapedProduced int
	shapedOut := make([]byte, uniformChunkLen)
	for i := 0; i < uniformChunkCount; i++ {
		// If we can't read any more random bytes, just reuse the existing ones---this is only a
		// rough statistical test, so that should be okay so long as the chunks are long enough.
		_, _ = cryptoRand.Read(uniformSource)
		tail := uniformSource
		for len(tail) > 0 {
			dn, sn := enc.ShapeBytes(shapedOut, tail)
			switch {
			case dn == 0 && sn == 0:
				t.Fatalf("encoder made no progress in ShapeBytes")
			case dn < 0:
				t.Fatalf("encoder claims to have produced %d bytes", dn)
			case sn < 0:
				t.Fatalf("encoder claims to have produced %d bytes", sn)
			case dn > len(shapedOut):
				t.Fatalf("encoder claims to have produced %d > %d bytes", dn, len(shapedOut))
			case sn > len(tail):
				t.Fatalf("encoder claims to have consumed %d > %d bytes", sn, len(tail))
			}

			for _, b := range shapedOut[:dn] {
				shapedDist[b]++
			}

			sourceConsumed += sn
			shapedProduced += dn
			tail = tail[sn:]
		}
	}

	if sourceConsumed != totalSource {
		t.Logf("somehow consumed %d != %d bytes?!", sourceConsumed, totalSource)
	}

	expansionRatio := float64(shapedProduced) / float64(sourceConsumed)
	expansionPercent := 100.0 * (expansionRatio - 1.0)
	t.Logf("simulated average shaped/uniform expansion: %+2.f%% (%d / %d)", expansionPercent, shapedProduced, sourceConsumed)
	t.Logf("expected uniform transfer rate: %0.2e B/s = %0.2e b/s", bytesPerSecond / expansionRatio, bitsPerSecond / expansionRatio)

	var shapedDistDisplay [256]string
	for i, n := range shapedDist {
		negLog := -math.Log2(float64(n) / float64(shapedProduced))
		if math.IsInf(negLog, 0) {
			shapedDistDisplay[i] = "*"
		} else {
			shapedDistDisplay[i] = fmt.Sprintf("%0.1f", negLog)
		}
	}
	t.Logf("shaped byte distribution -log: %s", strings.Join(shapedDistDisplay[:], " "))
}

func TestOneDirection(t *testing.T, enc Encoder, dec Decoder) {
	TestExpectedPerformance(t, enc)
}
