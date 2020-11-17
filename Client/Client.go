package main

import (
	"context"
	"flag"
	"net"
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wav"
	"github.com/moutend/go-wca/pkg/wca"
)

var verbose bool
var addr string

func main() {
	var err error
	var audio *wav.File
	flag.StringVar(&addr, "e", "", "server address to connect to \nex: -e 127.0.0.1:4040")
	flag.BoolVar(&verbose, "v", false, "displays more info about the stream")
	flag.Parse()
	if addr == "" {
		println("You have to specify the service address endpoint")
		println("ex: -e 127.0.0.1:4040")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 1)
	go func() {
		select {
		case <-signalChan:
			cancel()
			os.Exit(1)
		}

	}()

	audio, err = wav.New(48000, 16, 2)
	checkError(err)

	err = renderSharedTimerDriven(ctx, audio)
	checkError(err)
}

func checkError(err error) {
	if err != nil {
		println(err)
		os.Exit(1)
	}
}

func renderSharedTimerDriven(ctx context.Context, audio *wav.File) (err error) {
	err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	checkError(err)
	defer ole.CoUninitialize()

	var de *wca.IMMDeviceEnumerator
	err = wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &de)
	checkError(err)
	defer de.Release()

	var mmd *wca.IMMDevice
	err = de.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd)
	checkError(err)
	defer mmd.Release()

	var ps *wca.IPropertyStore
	err = mmd.OpenPropertyStore(wca.STGM_READ, &ps)
	checkError(err)
	defer ps.Release()

	var pv wca.PROPVARIANT
	err = ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv)
	checkError(err)
	println("Rendering audio to: " + pv.String())

	var ac3 *wca.IAudioClient3
	err = mmd.Activate(wca.IID_IAudioClient3, wca.CLSCTX_ALL, nil, &ac3)
	checkError(err)
	defer ac3.Release()

	var wfx *wca.WAVEFORMATEX
	err = ac3.GetMixFormat(&wfx)
	checkError(err)
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(wfx)))

	wfx.WFormatTag = 1
	wfx.NSamplesPerSec = uint32(audio.SamplesPerSec())
	wfx.WBitsPerSample = uint16(audio.BitsPerSample())
	wfx.NChannels = uint16(audio.Channels())
	wfx.NBlockAlign = uint16(audio.BlockAlign())
	wfx.NAvgBytesPerSec = uint32(audio.AvgBytesPerSec())
	wfx.CbSize = 0

	if verbose {
		println("--------")
		println("Format: PCM " + strconv.Itoa(int(wfx.WBitsPerSample)) + " bit signed integer")
		println("Rate: " + strconv.Itoa(int(wfx.NSamplesPerSec)) + " Hz")
		println("Channels: " + strconv.Itoa(int(wfx.NChannels)))
		println("--------")
	}
	var defaultPeriodInFrames, fundamentalPeriodInFrames, minPeriodInFrames, maxPeriodInFrames uint32
	err = ac3.GetSharedModeEnginePeriod(wfx, &defaultPeriodInFrames, &fundamentalPeriodInFrames, &minPeriodInFrames, &maxPeriodInFrames)
	checkError(err)

	if verbose {
		println("Default period in frames: " + strconv.Itoa(int(defaultPeriodInFrames)))
		println("Fundamental period in frames: " + strconv.Itoa(int(fundamentalPeriodInFrames)))
		println("Min period in frames: " + strconv.Itoa(int(minPeriodInFrames)))
		println("Max period in frames: " + strconv.Itoa(int(maxPeriodInFrames)))
	}

	var latency time.Duration = time.Duration(float64(minPeriodInFrames)/float64(wfx.NSamplesPerSec)*1000) * time.Millisecond
	err = ac3.InitializeSharedAudioStream(wca.AUDCLNT_SHAREMODE_SHARED, minPeriodInFrames, wfx, nil)
	checkError(err)

	var bufferFrameSize uint32
	err = ac3.GetBufferSize(&bufferFrameSize)
	checkError(err)

	if verbose {
		println("Allocated buffer size: " + strconv.Itoa(int(bufferFrameSize)))
		println("Latency: ", latency.String())
		println("--------")
	}

	var arc *wca.IAudioRenderClient
	err = ac3.GetService(wca.IID_IAudioRenderClient, &arc)
	checkError(err)
	defer arc.Release()

	err = ac3.Start()
	checkError(err)

	println("Start rendering audio with shared-timer-driven mode")
	println("Press Ctrl-C to stop rendering")

	time.Sleep(latency)

	var data *byte
	var padding uint32
	var availableFrameSize uint32
	var b *byte
	var start = unsafe.Pointer(data)
	var lim = int(availableFrameSize) * int(wfx.NBlockAlign)
	var buf []byte

	raddr, err := net.ResolveTCPAddr("tcp", addr)
	checkError(err)
	conn, err := net.DialTCP("tcp", nil, raddr)
	checkError(err)
	defer conn.Close()

	init := true
	for {
		err = ac3.GetCurrentPadding(&padding)
		checkError(err)
		availableFrameSize = bufferFrameSize - padding
		err = arc.GetBuffer(availableFrameSize, &data)
		checkError(err)
		start = unsafe.Pointer(data)
		lim = int(availableFrameSize) * int(wfx.NBlockAlign)
		buf = make([]byte, lim)
		if init {
			for i := 0; i < 100; i++ {
				_, err = conn.Read(buf)
				checkError(err)
			}
			init = false
		}
		_, err = conn.Read(buf)
		checkError(err)
		for n := 0; n < lim; n++ {
			b = (*byte)(unsafe.Pointer(uintptr(start) + uintptr(n)))
			*b = buf[n]
		}
		err = arc.ReleaseBuffer(availableFrameSize, 0)
		checkError(err)
		time.Sleep(latency / 2)
	}
}
