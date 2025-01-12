package policy

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benthosdev/benthos/v4/internal/manager/mock"
	"github.com/benthosdev/benthos/v4/internal/message"
	"github.com/benthosdev/benthos/v4/internal/old/processor"
)

func TestPolicyNoop(t *testing.T) {
	conf := NewConfig()
	assert.True(t, conf.IsNoop())

	conf = NewConfig()
	conf.Count = 2
	assert.False(t, conf.IsNoop())

	conf = NewConfig()
	conf.Check = "foo.bar"
	assert.False(t, conf.IsNoop())

	conf = NewConfig()
	conf.ByteSize = 10
	assert.False(t, conf.IsNoop())

	conf = NewConfig()
	conf.Period = "10s"
	assert.False(t, conf.IsNoop())
}

func TestPolicyBasic(t *testing.T) {
	conf := NewConfig()
	conf.Count = 2
	conf.ByteSize = 0

	pol, err := New(conf, mock.NewManager())
	require.NoError(t, err)

	t.Cleanup(func() {
		pol.CloseAsync()
		require.NoError(t, pol.WaitForClose(time.Second))
	})

	if v := pol.UntilNext(); v >= 0 {
		t.Errorf("Non-negative period: %v", v)
	}

	if exp, act := 0, pol.Count(); exp != act {
		t.Errorf("Wrong count: %v != %v", act, exp)
	}

	exp := [][]byte{[]byte("foo"), []byte("bar")}

	if pol.Add(message.NewPart(exp[0])) {
		t.Error("Unexpected batch")
	}
	if exp, act := 1, pol.Count(); exp != act {
		t.Errorf("Wrong count: %v != %v", act, exp)
	}
	if !pol.Add(message.NewPart(exp[1])) {
		t.Error("Expected batch")
	}
	if exp, act := 2, pol.Count(); exp != act {
		t.Errorf("Wrong count: %v != %v", act, exp)
	}

	msg := pol.Flush()
	if !reflect.DeepEqual(exp, message.GetAllBytes(msg)) {
		t.Errorf("Wrong result: %s != %s", message.GetAllBytes(msg), exp)
	}
	if exp, act := 0, pol.Count(); exp != act {
		t.Errorf("Wrong count: %v != %v", act, exp)
	}

	if msg = pol.Flush(); msg != nil {
		t.Error("Non-nil empty flush")
	}
}

func TestPolicyPeriod(t *testing.T) {
	conf := NewConfig()
	conf.Period = "300ms"

	pol, err := New(conf, mock.NewManager())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pol.CloseAsync()
		require.NoError(t, pol.WaitForClose(time.Second))
	})

	if pol.Add(message.NewPart(nil)) {
		t.Error("Unexpected batch ready")
	}

	if v := pol.UntilNext(); v >= (time.Millisecond*300) || v < (time.Millisecond*100) {
		t.Errorf("Wrong period: %v", v)
	}

	<-time.After(time.Millisecond * 500)
	if v := pol.UntilNext(); v >= (time.Millisecond * 100) {
		t.Errorf("Wrong period: %v", v)
	}

	if v := pol.Flush(); v == nil {
		t.Error("Nil msgs from flush")
	}

	if v := pol.UntilNext(); v >= (time.Millisecond*300) || v < (time.Millisecond*100) {
		t.Errorf("Wrong period: %v", v)
	}
}

func TestPolicySize(t *testing.T) {
	conf := NewConfig()
	conf.ByteSize = 10

	pol, err := New(conf, mock.NewManager())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pol.CloseAsync()
		require.NoError(t, pol.WaitForClose(time.Second))
	})

	exp := [][]byte{[]byte("foo bar"), []byte("baz qux")}

	if pol.Add(message.NewPart(exp[0])) {
		t.Error("Unexpected batch")
	}
	if !pol.Add(message.NewPart(exp[1])) {
		t.Error("Expected batch")
	}

	msg := pol.Flush()
	if !reflect.DeepEqual(exp, message.GetAllBytes(msg)) {
		t.Errorf("Wrong result: %s != %s", message.GetAllBytes(msg), exp)
	}

	if msg = pol.Flush(); msg != nil {
		t.Error("Non-nil empty flush")
	}
}

func TestPolicyCheck(t *testing.T) {
	conf := NewConfig()
	conf.Check = `content() == "bar"`

	pol, err := New(conf, mock.NewManager())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pol.CloseAsync()
		require.NoError(t, pol.WaitForClose(time.Second))
	})

	exp := [][]byte{[]byte("foo"), []byte("bar")}

	if pol.Add(message.NewPart(exp[0])) {
		t.Error("Unexpected batch")
	}
	if !pol.Add(message.NewPart(exp[1])) {
		t.Error("Expected batch")
	}

	msg := pol.Flush()
	if !reflect.DeepEqual(exp, message.GetAllBytes(msg)) {
		t.Errorf("Wrong result: %s != %s", message.GetAllBytes(msg), exp)
	}

	if msg = pol.Flush(); msg != nil {
		t.Error("Non-nil empty flush")
	}
}

func TestPolicyCheckAdvanced(t *testing.T) {
	conf := NewConfig()
	conf.Check = `batch_size() >= 3`

	pol, err := New(conf, mock.NewManager())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pol.CloseAsync()
		require.NoError(t, pol.WaitForClose(time.Second))
	})

	exp := [][]byte{[]byte("foo"), []byte("bar"), []byte("baz")}

	if pol.Add(message.NewPart(exp[0])) {
		t.Error("Unexpected batch")
	}
	if pol.Add(message.NewPart(exp[1])) {
		t.Error("Expected batch")
	}
	if !pol.Add(message.NewPart(exp[2])) {
		t.Error("Expected batch")
	}

	msg := pol.Flush()
	if !reflect.DeepEqual(exp, message.GetAllBytes(msg)) {
		t.Errorf("Wrong result: %s != %s", message.GetAllBytes(msg), exp)
	}

	if msg = pol.Flush(); msg != nil {
		t.Error("Non-nil empty flush")
	}
}

func TestPolicyArchived(t *testing.T) {
	conf := NewConfig()
	conf.Count = 2
	conf.ByteSize = 0

	procConf := processor.NewConfig()
	procConf.Type = processor.TypeArchive
	procConf.Archive.Format = "lines"

	conf.Processors = append(conf.Processors, procConf)

	pol, err := New(conf, mock.NewManager())
	require.NoError(t, err)

	t.Cleanup(func() {
		pol.CloseAsync()
		require.NoError(t, pol.WaitForClose(time.Second))
	})

	exp := [][]byte{[]byte("foo\nbar")}

	assert.False(t, pol.Add(message.NewPart([]byte("foo"))))
	assert.Equal(t, 1, pol.Count())

	assert.True(t, pol.Add(message.NewPart([]byte("bar"))))
	assert.Equal(t, 2, pol.Count())

	msg := pol.Flush()
	assert.Equal(t, exp, message.GetAllBytes(msg))
	assert.Equal(t, 0, pol.Count())

	msg = pol.Flush()
	assert.Nil(t, msg)
}

func TestPolicySplit(t *testing.T) {
	conf := NewConfig()
	conf.Count = 2
	conf.ByteSize = 0

	procConf := processor.NewConfig()
	procConf.Type = processor.TypeSplit

	conf.Processors = append(conf.Processors, procConf)

	pol, err := New(conf, mock.NewManager())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pol.CloseAsync()
		require.NoError(t, pol.WaitForClose(time.Second))
	})

	exp := [][]byte{[]byte("foo"), []byte("bar")}

	if pol.Add(message.NewPart([]byte("foo"))) {
		t.Error("Unexpected batch")
	}
	if exp, act := 1, pol.Count(); exp != act {
		t.Errorf("Wrong count: %v != %v", act, exp)
	}
	if !pol.Add(message.NewPart([]byte("bar"))) {
		t.Error("Expected batch")
	}
	if exp, act := 2, pol.Count(); exp != act {
		t.Errorf("Wrong count: %v != %v", act, exp)
	}

	msg := pol.Flush()
	if !reflect.DeepEqual(exp, message.GetAllBytes(msg)) {
		t.Errorf("Wrong result: %s != %s", message.GetAllBytes(msg), exp)
	}
	if exp, act := 0, pol.Count(); exp != act {
		t.Errorf("Wrong count: %v != %v", act, exp)
	}

	if msg = pol.Flush(); msg != nil {
		t.Error("Non-nil empty flush")
	}
}
