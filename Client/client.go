package main

import (
	"flag"
	"fmt"
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
var guiMode bool

func main() {
	var addr string
	flag.BoolVar(&guiMode, "cli", false, "start cli mode")
	flag.StringVar(&addr, "e", "", "server address to connect to \nex: -e 127.0.0.1:4040")
	flag.BoolVar(&verbose, "v", false, "displays more info about the stream")
	flag.Parse()
	guiMode = !guiMode

	if guiMode {
		startGUI()
	} else {
		//Look at function for explanation
		allocNewConsole()

		if addr == "" {
			println("You have to specify the service address endpoint")
			println("ex: -e 127.0.0.1:4040")
			enterToEndCLI()
			os.Exit(0)
		}

		var disconnect bool = false
		_ = audioStartup(addr, &disconnect)
		signalChan := make(chan os.Signal, 1)
		<-signalChan
		disconnect = true
		os.Exit(0)
	}
}

func audioStartup(addr string, disconnect *bool) *int {
	var err error
	var audio *wav.File
	var connStatus int = 3
	audio, err = wav.New(48000, 16, 2)
	checkError(err)
	go func() {
		err = renderSharedTimerDriven(audio, addr, &connStatus, &disconnect)
		checkError(err)
	}()
	return &connStatus
}

func checkError(err error) {
	if err != nil {
		if !guiMode {
			println(fmt.Sprint(err))
			enterToEndCLI()
			os.Exit(0)
		}
		println(fmt.Sprint(err))
	}
}

func renderSharedTimerDriven(audio *wav.File, addr string, connStatus *int, disconnect **bool) (err error) {
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

	//TCP CONNECTION
	raddr, err := net.ResolveTCPAddr("tcp", addr)
	checkError(err)
	if err != nil {
		*connStatus = 6
		return
	}
	conn, err := net.DialTCP("tcp", nil, raddr)
	checkError(err)
	if **disconnect {
		return
	}
	if err != nil {
		*connStatus = 2
		return
	}
	defer conn.Close()

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

	var data *byte
	var padding uint32
	var availableFrameSize uint32
	var b *byte
	var start = unsafe.Pointer(data)
	var lim = int(availableFrameSize) * int(wfx.NBlockAlign)
	var buf []byte

	//Reduce latency by skipping over the first parts of audio
	//Recalculate lim since it usually starts with 0 -> wouldn't skip any audio
	*connStatus = 5
	for i := 0; i < 200; i++ {
		availableFrameSize = bufferFrameSize - padding
		lim = int(availableFrameSize) * int(wfx.NBlockAlign)
		skip := make([]byte, lim)
		_, err = conn.Read(skip)
		checkError(err)
		if err != nil {
			*connStatus = 4
			return
		}
		if **disconnect {
			return
		}
	}
	*connStatus = 1

	for {
		if **disconnect {
			println("Disconnected")
			return
		}
		err = ac3.GetCurrentPadding(&padding)
		availableFrameSize = bufferFrameSize - padding
		err = arc.GetBuffer(availableFrameSize, &data)
		start = unsafe.Pointer(data)
		lim = int(availableFrameSize) * int(wfx.NBlockAlign)
		buf = make([]byte, lim)
		_, err = conn.Read(buf)
		checkError(err)
		if err != nil {
			*connStatus = 4
			break
		}

		for n := 0; n < lim && n < len(buf); n++ {
			b = (*byte)(unsafe.Pointer(uintptr(start) + uintptr(n)))
			*b = buf[n]
		}
		err = arc.ReleaseBuffer(availableFrameSize, 0)
		checkError(err)
		time.Sleep(latency / 2)
	}
	return
}
