// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.
// +build windows

package pdhutil

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type PdhFormatter struct {
	buf []uint8
}

type PdhCounterValue struct {
	Format  uint32
	CStatus uint32
	Double  float64
	Large   int64
	Long    int32
}

type ValueEnumFunc func(s string, v PdhCounterValue)

func (f *PdhFormatter) Enum(hCounter PDH_HCOUNTER, format uint32, fn ValueEnumFunc) error {
	var bufLen uint32
	var itemCount uint32
	r, _, _ := procPdhGetFormattedCounterArray.Call(
		uintptr(hCounter),
		uintptr(format),
		uintptr(unsafe.Pointer(&bufLen)),
		uintptr(unsafe.Pointer(&itemCount)),
		uintptr(0),
	)

	if r != PDH_MORE_DATA {
		return fmt.Errorf("Failed to get formatted counter array buffer size 0x%x", r)
	}

	if bufLen > uint32(len(f.buf)) {
		f.buf = make([]uint8, bufLen)
	}

	buf := f.buf[:bufLen]

	r, _, _ = procPdhGetFormattedCounterArray.Call(
		uintptr(hCounter),
		uintptr(format),
		uintptr(unsafe.Pointer(&bufLen)),
		uintptr(unsafe.Pointer(&itemCount)),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r != ERROR_SUCCESS {
		return fmt.Errorf("Error getting formatted counter array 0x%x", r)
	}

	items := (*[1 << 29]PDH_FMT_COUNTERVALUE_ITEM)(unsafe.Pointer(&buf[0]))[:itemCount:itemCount]

	var (
		prevName    string
		instanceIdx int
	)
	for _, item := range items {
		// TODO - cleanup the logic here - len should be decreasing on every item to be within
		// bounds of the allocated buf
		u := (*[1 << 29]uint16)(unsafe.Pointer(item.szName))[: bufLen/2 : bufLen/2]
		for i, v := range u {
			if v == 0 {
				u = u[:i]
				break
			}
		}

		name := windows.UTF16ToString(u)
		if name != prevName {
			instanceIdx = 0
			prevName = name
		} else {
			instanceIdx++
		}

		value := formattedItemToValue(format, unsafe.Pointer(&item.value))
		fn(fmt.Sprintf("%s#%d", name, instanceIdx), value)
	}
	return nil
}

func formattedItemToValue(format uint32, p unsafe.Pointer) PdhCounterValue {
	value := PdhCounterValue{
		Format: format,
	}
	switch format {
	case PDH_FMT_DOUBLE:
		from := (*PDH_FMT_COUNTERVALUE_DOUBLE)(p)
		value.CStatus = from.CStatus
		value.Double = from.DoubleValue
	case PDH_FMT_LONG:
		from := (*PDH_FMT_COUNTERVALUE_LONG)(p)
		value.CStatus = from.CStatus
		value.Long = from.LongValue
	case PDH_FMT_LARGE:
		from := (*PDH_FMT_COUNTERVALUE_LARGE)(p)
		value.CStatus = from.CStatus
		value.Large = from.LargeValue
	}
	return value
}
