package buffer

import (
	"time"

	"github.com/stretchr/testify/mock"
)

// MockRingBufferInterface mocks RingBufferInterface
type MockRingBufferInterface struct {
	mock.Mock
}

func (m *MockRingBufferInterface) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockRingBufferInterface) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockRingBufferInterface) Length() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockRingBufferInterface) Capacity() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockRingBufferInterface) Free() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockRingBufferInterface) Reset() {
	m.Called()
}

// MockTimeProvider mocks TimeProvider
type MockTimeProvider struct {
	mock.Mock
}

func (m *MockTimeProvider) Now() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

func (m *MockTimeProvider) Sleep(d time.Duration) {
	m.Called(d)
}

// MockLogger mocks Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.Called(append([]interface{}{msg}, args...)...)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.Called(append([]interface{}{msg}, args...)...)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.Called(append([]interface{}{msg}, args...)...)
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.Called(append([]interface{}{msg}, args...)...)
}

// MockAnalysisBufferInterface mocks AnalysisBufferInterface
type MockAnalysisBufferInterface struct {
	mock.Mock
}

func (m *MockAnalysisBufferInterface) Write(data []byte) (int, error) {
	args := m.Called(data)
	return args.Int(0), args.Error(1)
}

func (m *MockAnalysisBufferInterface) Read(p []byte) (int, error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockAnalysisBufferInterface) Reset() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAnalysisBufferInterface) SampleRate() uint32 {
	args := m.Called()
	return uint32(args.Int(0))
}

func (m *MockAnalysisBufferInterface) Channels() uint32 {
	args := m.Called()
	return uint32(args.Int(0))
}

func (m *MockAnalysisBufferInterface) ReadyForAnalysis() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockCaptureBufferInterface mocks CaptureBufferInterface
type MockCaptureBufferInterface struct {
	mock.Mock
}

func (m *MockCaptureBufferInterface) Write(data []byte) (int, error) {
	args := m.Called(data)
	return args.Int(0), args.Error(1)
}

func (m *MockCaptureBufferInterface) Read(p []byte) (int, error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockCaptureBufferInterface) Reset() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCaptureBufferInterface) SampleRate() uint32 {
	args := m.Called()
	return uint32(args.Int(0))
}

func (m *MockCaptureBufferInterface) Channels() uint32 {
	args := m.Called()
	return uint32(args.Int(0))
}

func (m *MockCaptureBufferInterface) ReadSegment(startTime time.Time, durationSeconds int) ([]byte, error) {
	args := m.Called(startTime, durationSeconds)
	return args.Get(0).([]byte), args.Error(1)
}

// MockBufferFactory mocks BufferFactory for testing buffer manager
type MockBufferFactory struct {
	mock.Mock
}

func (m *MockBufferFactory) CreateAnalysisBuffer(
	sampleRate, channels uint32,
	duration time.Duration,
) AnalysisBufferInterface {
	args := m.Called(sampleRate, channels, duration)
	return args.Get(0).(AnalysisBufferInterface)
}

func (m *MockBufferFactory) CreateCaptureBuffer(
	sampleRate, channels uint32,
	duration time.Duration,
) CaptureBufferInterface {
	args := m.Called(sampleRate, channels, duration)
	return args.Get(0).(CaptureBufferInterface)
}

func (m *MockBufferFactory) CreateBufferManager() BufferManagerInterface {
	args := m.Called()
	return args.Get(0).(BufferManagerInterface)
}
