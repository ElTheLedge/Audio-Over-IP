package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"time"
	"unsafe"

	"github.com/LVH-IT/mygotools"
	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

var listenPort *string

func main() {
	listenPort = flag.String("p", "4040", "Port to listen on")
	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		select {
		case <-signalChan:
			os.Exit(1)
		}
	}()
	for {
		loopbackCaptureSharedTimerDriven()
		time.Sleep(100 * time.Millisecond)
		mygotools.ClearCLI()
	}

}

func checkError(err error) {
	if err != nil {
		_, fn, line, _ := runtime.Caller(1)
		log.Printf("[error] %s:%d %v", fn, line, err)
	}
}

func loopbackCaptureSharedTimerDriven() {
	var err error
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
	println("Capturing audio from: " + pv.String())

	var ac *wca.IAudioClient
	err = mmd.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &ac)
	checkError(err)
	defer ac.Release()

	var wfx *wca.WAVEFORMATEX
	err = ac.GetMixFormat(&wfx)
	checkError(err)
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(wfx)))

	println("--------")
	println("Format: PCM " + strconv.Itoa(int(wfx.WBitsPerSample)) + " bit signed integer")
	println("Rate: " + strconv.Itoa(int(wfx.NSamplesPerSec)) + " Hz")
	println("Channels: " + strconv.Itoa(int(wfx.NChannels)))
	println("--------")

	var defaultPeriod wca.REFERENCE_TIME
	var minimumPeriod wca.REFERENCE_TIME
	var latency time.Duration
	err = ac.GetDevicePeriod(&defaultPeriod, &minimumPeriod)
	checkError(err)

	latency = time.Duration(int(defaultPeriod) * 100)

	println("Default period: " + strconv.Itoa(int(defaultPeriod)))
	println("Minimum period: " + strconv.Itoa(int(minimumPeriod)))
	println("Latency: " + latency.String())

	err = ac.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, wca.AUDCLNT_STREAMFLAGS_LOOPBACK, wca.REFERENCE_TIME(400*10000), 0, wfx, nil)
	checkError(err)

	var bufferFrameSize uint32
	err = ac.GetBufferSize(&bufferFrameSize)
	checkError(err)
	println("Allocated buffer size: " + strconv.Itoa(int(bufferFrameSize)))

	var acc *wca.IAudioCaptureClient
	ac.GetService(wca.IID_IAudioCaptureClient, &acc)
	checkError(err)
	defer acc.Release()

	//TCP LISTENER
	laddr, err := net.ResolveTCPAddr("tcp", ":"+*listenPort)
	checkError(err)
	l, err := net.ListenTCP("tcp", laddr)
	checkError(err)
	defer l.Close()
	println("listening at (tcp) " + l.Addr().String())
	conn, err := l.AcceptTCP()
	checkError(err)
	println("Client connected (" + conn.RemoteAddr().String() + ")")

	err = ac.Start()
	checkError(err)
	println("Start loopback capturing with shared timer driven mode")
	println("Press Ctrl-C to stop capturing")

	var data *byte
	var b *byte
	var availableFrameSize uint32
	var flags uint32
	var devicePosition uint64
	var qcpPosition uint64

	for {
		acc.GetBuffer(&data, &availableFrameSize, &flags, &devicePosition, &qcpPosition)
		start := unsafe.Pointer(data)
		lim := int(availableFrameSize) * int(wfx.NBlockAlign)
		buf := make([]byte, lim)

		for n := 0; n < lim; n++ {
			b = (*byte)(unsafe.Pointer(uintptr(start) + uintptr(n)))
			buf[n] = *b
		}

		_, err = conn.Write(buf)
		if err != nil {
			println("Client at " + conn.RemoteAddr().String() + " disconnected")
			break
		}
		acc.ReleaseBuffer(availableFrameSize)

		//Lower CPU usage (from 10% to 0.x%) but higher latency
		time.Sleep(latency / 2)
	}
}
