package netpunchlib_test

import (
	"errors"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/michurin/netpunch/netpunchlib"
	"github.com/michurin/netpunch/netpunchlib/internal/mock"
)

func TestClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mock.NewMockConnection(ctrl)
	m.EXPECT().Close().Return(nil)

	conn := netpunchlib.SigningMiddleware([]byte("MORN"))(m)
	err := conn.Close()

	assert.NoError(t, err)
}

func TestWriteToUDP_ok(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mock.NewMockConnection(ctrl)
	m.EXPECT().WriteToUDP([]byte("@9d*O[`bg>M-oOn?)ikhf%&gWemV?-5#T/G data"), nil).Return(40, nil)

	conn := netpunchlib.SigningMiddleware([]byte("MORN"))(m)
	n, err := conn.WriteToUDP([]byte("data"), nil)

	assert.Equal(t, 4, n)
	assert.NoError(t, err)
}

func TestWriteToUDP_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mock.NewMockConnection(ctrl)
	m.EXPECT().WriteToUDP([]byte("@9d*O[`bg>M-oOn?)ikhf%&gWemV?-5#T/G data"), nil).Return(0, errors.New("TestErr"))

	conn := netpunchlib.SigningMiddleware([]byte("MORN"))(m)
	n, err := conn.WriteToUDP([]byte("data"), nil)

	assert.Equal(t, 0, n)
	assert.Errorf(t, err, "TestErr")
}

func TestReadFromUDP_ok(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mock.NewMockConnection(ctrl)
	m.EXPECT().ReadFromUDP(gomock.Any()).DoAndReturn(func(b []byte) (int, *net.UDPAddr, error) {
		copy(b, []byte("@9d*O[`bg>M-oOn?)ikhf%&gWemV?-5#T/G data"))
		return 40, nil, nil
	})

	conn := netpunchlib.SigningMiddleware([]byte("MORN"))(m)
	buff := make([]byte, 1024)
	n, addr, err := conn.ReadFromUDP(buff)

	assert.Equal(t, 4, n)
	assert.Equal(t, []byte("data"), buff[:n])
	assert.Nil(t, addr)
	assert.NoError(t, err)
}
