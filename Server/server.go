package main

import (
	"bufio"
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wav"
	"github.com/moutend/go-wca/pkg/wca"
)

func main() {
	var err error
	var durationFlag time.Duration
	durationFlag = 0
	println(durationFlag)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	ctx, _ := context.WithCancel(context.Background())
	go func() {
		select {
		case <-signalChan:
			os.Exit(1)
		}
	}()
	_, err = loopbackCaptureSharedTimerDriven(ctx, durationFlag)
	checkError(err)
	println("End of main() function")
}

func checkError(err error) {
	if err != nil {
		println(err)
		os.Exit(1)
	}
}

func loopbackCaptureSharedTimerDriven(ctx context.Context, duration time.Duration) (audio *wav.File, err error) {
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

	wfx.WFormatTag = 1
	wfx.WBitsPerSample = 16
	wfx.NBlockAlign = (wfx.WBitsPerSample / 8) * wfx.NChannels // 16 bit stereo is 32bit (4 byte) per sample
	wfx.NAvgBytesPerSec = wfx.NSamplesPerSec * uint32(wfx.NBlockAlign)
	wfx.CbSize = 0

	audio, err = wav.New(int(wfx.NSamplesPerSec), int(wfx.WBitsPerSample), int(wfx.NChannels))
	checkError(err)

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

	//latency = time.Duration(int(defaultPeriod) * 100)
	latency = time.Duration(int(defaultPeriod) * 10)

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

	err = ac.Start()
	checkError(err)
	println("Start loopback capturing with shared timer driven mode")
	if duration <= 0 {
		println("Press Ctrl-C to stop capturing")
	}
	time.Sleep(latency)

	var data *byte
	var b *byte
	var availableFrameSize uint32
	var flags uint32
	var devicePosition uint64
	var qcpPosition uint64

	var addr string = ":4040"
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	checkError(err)
	l, err := net.ListenTCP("tcp", laddr)
	checkError(err)
	defer l.Close()
	println("listening at (tcp) " + laddr.String())
	conn, err := l.AcceptTCP()
	checkError(err)
	println("connected to: " + conn.RemoteAddr().String())

	w := bufio.NewWriter(conn)
	for {
		acc.GetBuffer(&data, &availableFrameSize, &flags, &devicePosition, &qcpPosition)
		start := unsafe.Pointer(data)
		lim := int(availableFrameSize) * int(wfx.NBlockAlign)
		buf := make([]byte, int((float32(lim)*2)/2))

		for n := 0; n < lim; n++ {
			b = (*byte)(unsafe.Pointer(uintptr(start) + uintptr(n)))
			buf[n] = *b
		}
		_, err = w.Write(buf)
		checkError(err)
		w.Flush()
		acc.ReleaseBuffer(availableFrameSize)
		time.Sleep(latency / 4)
	}
}
