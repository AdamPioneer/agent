//
// Copyright (c) 2018-2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	pb "github.com/kata-containers/agent/protocols/grpc"
	"github.com/stretchr/testify/assert"
)

func createSafeAndFakeStorage() (pb.Storage, error) {
	dirPath, err := ioutil.TempDir("", "fake-dir")
	if err != nil {
		return pb.Storage{}, err
	}

	return pb.Storage{
		Source:     dirPath,
		MountPoint: filepath.Join(dirPath, "test-mount"),
	}, nil
}

func TestEphemeralStorageHandlerSuccessful(t *testing.T) {
	skipUnlessRoot(t)

	storage, err := createSafeAndFakeStorage()
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Unmount(storage.MountPoint, 0)
	defer os.RemoveAll(storage.MountPoint)

	storage.Fstype = typeTmpFs
	storage.Source = typeTmpFs
	sbs := make(map[string]*sandboxStorage)
	_, err = ephemeralStorageHandler(storage, &sandbox{storages: sbs})
	assert.Nil(t, err, "ephemeralStorageHandler() failed: %v", err)
}

func TestLocalStorageHandlerSuccessful(t *testing.T) {
	skipUnlessRoot(t)

	storage, err := createSafeAndFakeStorage()
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Unmount(storage.MountPoint, 0)
	defer os.RemoveAll(storage.MountPoint)

	sbs := make(map[string]*sandboxStorage)
	_, err = localStorageHandler(storage, &sandbox{storages: sbs})
	assert.Nil(t, err, "localStorageHandler() failed: %v", err)
}

func TestLocalStorageHandlerPermModeSuccessful(t *testing.T) {
	skipUnlessRoot(t)

	storage, err := createSafeAndFakeStorage()
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Unmount(storage.MountPoint, 0)
	defer os.RemoveAll(storage.MountPoint)

	// Set the mode to be 0400 (ready only)
	storage.Options = []string{
		"mode=0400",
	}

	sbs := make(map[string]*sandboxStorage)
	_, err = localStorageHandler(storage, &sandbox{storages: sbs})
	assert.Nil(t, err, "localStorageHandler() failed: %v", err)

	// Check the mode of the mountpoint
	info, err := os.Stat(storage.MountPoint)
	assert.Nil(t, err)
	assert.Equal(t, 0400|os.ModeDir, info.Mode())
}

func TestLocalStorageHandlerPermModeFailure(t *testing.T) {
	skipUnlessRoot(t)

	storage, err := createSafeAndFakeStorage()
	if err != nil {
		t.Fatal(err)
	}
	//defer syscall.Unmount(storage.MountPoint, 0)
	//defer os.RemoveAll(storage.MountPoint)

	// Set the mode to something invalid
	storage.Options = []string{
		"mode=abcde",
	}

	sbs := make(map[string]*sandboxStorage)
	_, err = localStorageHandler(storage, &sandbox{storages: sbs})
	assert.NotNil(t, err, "localStorageHandler() should have failed")
}

func TestVirtio9pStorageHandlerSuccessful(t *testing.T) {
	skipUnlessRoot(t)

	storage, err := createSafeAndFakeStorage()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(storage.Source)
	defer syscall.Unmount(storage.MountPoint, 0)

	storage.Fstype = "bind"
	storage.Options = []string{"rbind"}

	_, err = virtio9pStorageHandler(storage, &sandbox{})
	assert.Nil(t, err, "storage9pDriverHandler() failed: %v", err)
}

func TestVirtioBlkStoragePathFailure(t *testing.T) {
	s := &sandbox{}

	storage := pb.Storage{
		Source: "/home/developer/test",
	}

	_, err := virtioBlkStorageHandler(storage, s)
	agentLog.WithError(err).Error("virtioBlkStorageHandler error")
	assert.NotNil(t, err, "virtioBlkStorageHandler() should have failed")
}

func TestVirtioBlkStorageDeviceFailure(t *testing.T) {
	s := &sandbox{}

	storage := pb.Storage{
		Source: "/dev/foo",
	}

	_, err := virtioBlkStorageHandler(storage, s)
	agentLog.WithError(err).Error("virtioBlkStorageHandler error")
	assert.NotNil(t, err, "virtioBlkStorageHandler() should have failed")
}

func TestVirtioBlkStorageHandlerSuccessful(t *testing.T) {
	skipUnlessRoot(t)

	testDir, err := ioutil.TempDir("", "kata-agent-tmp-")
	if err != nil {
		t.Fatal(t, err)
	}

	bridgeID := "02"
	deviceID := "03"
	pciBus := "0000:01"
	completePCIAddr := fmt.Sprintf("0000:00:%s.0/%s:%s.0", bridgeID, pciBus, deviceID)

	pciID := fmt.Sprintf("%s/%s", bridgeID, deviceID)

	sysBusPrefix = testDir
	bridgeBusPath := fmt.Sprintf(pciBusPathFormat, sysBusPrefix, "0000:00:02.0")

	err = os.MkdirAll(filepath.Join(bridgeBusPath, pciBus), mountPerm)
	assert.Nil(t, err)

	devPath, err := createFakeDevicePath()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(devPath)

	dirPath, err := ioutil.TempDir("", "fake-dir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dirPath)

	storage := pb.Storage{
		Source:     pciID,
		MountPoint: filepath.Join(dirPath, "test-mount"),
	}
	defer syscall.Unmount(storage.MountPoint, 0)

	s := &sandbox{
		pciDeviceMap: make(map[string]string),
	}

	s.Lock()
	s.pciDeviceMap[completePCIAddr] = devPath
	s.Unlock()

	storage.Fstype = "bind"
	storage.Options = []string{"rbind"}

	systemDevPath = ""
	_, err = virtioBlkStorageHandler(storage, s)
	assert.Nil(t, err, "storageBlockStorageDriverHandler() failed: %v", err)
}

func testAddStoragesSuccessful(t *testing.T, storages []*pb.Storage) {
	_, err := addStorages(context.Background(), storages, &sandbox{})
	assert.Nil(t, err, "addStorages() failed: %v", err)
}

func TestAddStoragesEmptyStoragesSuccessful(t *testing.T) {
	var storages []*pb.Storage

	testAddStoragesSuccessful(t, storages)
}

func TestAddStoragesNilStoragesSuccessful(t *testing.T) {
	storages := []*pb.Storage{
		nil,
	}

	testAddStoragesSuccessful(t, storages)
}

func noopStorageHandlerReturnNil(storage pb.Storage, s *sandbox) (string, error) {
	return "", nil
}

func noopStorageHandlerReturnError(storage pb.Storage, s *sandbox) (string, error) {
	return "", fmt.Errorf("Noop handler failure")
}

func TestAddStoragesNoopHandlerSuccessful(t *testing.T) {
	noopHandlerTag := "noop"
	storageHandlerList = map[string]storageHandler{
		noopHandlerTag: noopStorageHandlerReturnNil,
	}

	storages := []*pb.Storage{
		{
			Driver: noopHandlerTag,
		},
	}

	testAddStoragesSuccessful(t, storages)
}

func testAddStoragesFailure(t *testing.T, storages []*pb.Storage) {
	_, err := addStorages(context.Background(), storages, &sandbox{})
	assert.NotNil(t, err, "addStorages() should have failed")
}

func TestAddStoragesUnknownHandlerFailure(t *testing.T) {
	storageHandlerList = map[string]storageHandler{}

	storages := []*pb.Storage{
		{
			Driver: "unknown",
		},
	}

	testAddStoragesFailure(t, storages)
}

func TestAddStoragesNoopHandlerFailure(t *testing.T) {
	noopHandlerTag := "noop"
	storageHandlerList = map[string]storageHandler{
		noopHandlerTag: noopStorageHandlerReturnError,
	}

	storages := []*pb.Storage{
		{
			Driver: noopHandlerTag,
		},
	}

	testAddStoragesFailure(t, storages)
}

func TestMount(t *testing.T) {
	assert := assert.New(t)

	type testData struct {
		source      string
		destination string
		fsType      string
		flags       int
		options     string

		expectError bool
	}

	data := []testData{
		{"", "", "", 0, "", true},
		{"", "/foo", "9p", 0, "", true},
		{"proc", "", "9p", 0, "", true},
		{"proc", "/proc", "", 0, "", true},
	}

	for i, d := range data {
		err := mount(d.source, d.destination, d.fsType, d.flags, d.options)

		if d.expectError {
			assert.Errorf(err, "test %d (%+v)", i, d)
		} else {
			assert.NoErrorf(err, "test %d (%+v)", i, d)
		}
	}
}

func TestMountParseMountFlagsAndOptions(t *testing.T) {
	assert := assert.New(t)

	type testData struct {
		options []string

		expectedFlags   int
		expectedOptions string
	}

	// Start with some basic tests
	data := []testData{
		{[]string{}, 0, ""},
		{[]string{"moo"}, 0, "moo"},
		{[]string{"moo", "foo"}, 0, "moo,foo"},
		{[]string{"foo", "moo"}, 0, "foo,moo"},
	}

	// Add the expected flag handling tests
	for name, value := range flagList {
		td := testData{
			options:         []string{"foo", name, "bar"},
			expectedFlags:   value,
			expectedOptions: "foo,bar",
		}

		data = append(data, td)
	}

	for i, d := range data {
		msg := fmt.Sprintf("test[%d]: %+v\n", i, d)

		flags, options := parseMountFlagsAndOptions(d.options)

		assert.Equal(d.expectedFlags, flags, msg)
		assert.Equal(d.expectedOptions, options, msg)

	}
}
