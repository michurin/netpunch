package netpunchlib_test

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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

	require.NoError(t, err)
}

func TestWriteToUDP_ok(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mock.NewMockConnection(ctrl)
	m.EXPECT().WriteToUDP([]byte(`VS2/W:Yo^Bl5K]QY&_nAD;I>W!Xe!?PY"r>0pm"S data`), nil).Return(45, nil) // 45=40+1+4

	conn := netpunchlib.SigningMiddleware([]byte("MORN"))(m)
	n, err := conn.WriteToUDP([]byte("data"), nil)

	require.NoError(t, err)
	assert.Equal(t, 4, n)
}

func TestWriteToUDP_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mock.NewMockConnection(ctrl)
	m.EXPECT().WriteToUDP([]byte(`VS2/W:Yo^Bl5K]QY&_nAD;I>W!Xe!?PY"r>0pm"S data`), nil).Return(0, errors.New("TestErr"))

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
		n := copy(b, []byte(`VS2/W:Yo^Bl5K]QY&_nAD;I>W!Xe!?PY"r>0pm"S data`))
		assert.Equal(t, 45, n)
		return 45, nil, nil
	})

	conn := netpunchlib.SigningMiddleware([]byte("MORN"))(m)
	buff := make([]byte, 1024)
	n, addr, err := conn.ReadFromUDP(buff)

	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, []byte("data"), buff[:n])
	assert.Nil(t, addr)
}
