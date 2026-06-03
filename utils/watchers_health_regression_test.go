package utils

import (
	"testing"

	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestNanoIdGeneration(t *testing.T) {
	id1 := NanoId(8)
	id2 := NanoId(8)
	assert.Len(t, id1, 8)
	assert.Len(t, id2, 8)
	assert.NotEqual(t, id1, id2)
}

func TestNanoIdVariousLengths(t *testing.T) {
	for _, n := range []int{4, 8, 12, 16, 21} {
		id := NanoId(n)
		assert.Len(t, id, n, "length %d", n)
	}
}

func TestStringSetOperations(t *testing.T) {
	ss := NewStringSet("a", "b", "c")
	assert.Equal(t, 3, ss.Len())
	assert.True(t, ss.Has("a"))
	assert.True(t, ss.Has("b"))
	assert.True(t, ss.Has("c"))
	assert.False(t, ss.Has("d"))

	ss.Add("d", "e")
	assert.Equal(t, 5, ss.Len())
	assert.True(t, ss.Has("d"))
	assert.True(t, ss.Has("e"))

	ss.Delete("a")
	assert.Equal(t, 4, ss.Len())
	assert.False(t, ss.Has("a"))

	items := ss.Items()
	assert.Equal(t, []string{"b", "c", "d", "e"}, items)
}

func TestStringSetEmpty(t *testing.T) {
	ss := NewStringSet()
	assert.Equal(t, 0, ss.Len())
	assert.False(t, ss.Has("anything"))
	assert.Equal(t, "", ss.GetFirst())
	assert.Equal(t, []string{}, ss.Items())
}

func TestStringSetAddEmptyStrings(t *testing.T) {
	ss := NewStringSet()
	ss.Add("")
	assert.Equal(t, 0, ss.Len())
	ss.Add("a", "", "b")
	assert.Equal(t, 2, ss.Len())
}

func TestStringSetNilReceiver(t *testing.T) {
	var ss *StringSet
	assert.False(t, ss.Has("a"))
	assert.Equal(t, []string{}, ss.Items())
	assert.Equal(t, 0, ss.Len())
}

func TestGlobMatch(t *testing.T) {
	assert.True(t, GlobMatch("project.*", "project.*"))
	assert.True(t, GlobMatch("project.*", "project.settings"))
	assert.True(t, GlobMatch("*", "anything"))
	assert.True(t, GlobMatch("project.*", "project.alerts"))
	assert.False(t, GlobMatch("project.*", "users.edit"))
	assert.True(t, GlobMatch("project.application", "project.application"))
	assert.False(t, GlobMatch("project.application", "project.node"))
}

func TestGlobMatchWildcards(t *testing.T) {
	assert.True(t, GlobMatch("node-*", "node-1"))
	assert.True(t, GlobMatch("node-*", "node-prod"))
	assert.False(t, GlobMatch("node-*", "pod-1"))
	assert.True(t, GlobMatch("*-service", "user-service"))
	assert.True(t, GlobMatch("*-service", "payment-service"))
	assert.False(t, GlobMatch("*-service", "user-deployment"))
}

func TestLastPart(t *testing.T) {
	assert.Equal(t, "abc123", LastPart("nginx-deployment-abc123", "-"))
	assert.Equal(t, "v1", LastPart("app:v1", ":"))
	assert.Equal(t, "suffix", LastPart("prefix.suffix", "."))
	assert.Equal(t, "only", LastPart("only", "-"))
}

func TestFormatImage(t *testing.T) {
	assert.Equal(t, "catalog:0.33", FormatImage("docker.io/organization/catalog:0.33"))
	assert.Equal(t, "img:95048a46", FormatImage("docker.io/orgg/img:95048a46"))
	assert.Equal(t, "package-image@sha256:2d01d1a", FormatImage("repo.io/org/package-image@sha256:2d01d1af064c8cdb32f51406f4148091cd0c87168c41725a62110aae9a6a44b4"))
	assert.Equal(t, "nginx:latest", FormatImage("nginx:latest"))
	assert.Equal(t, "busybox:1.36", FormatImage("library/busybox:1.36"))
}

func TestFormatFloat(t *testing.T) {
	assert.Equal(t, "0", FormatFloat(0))
	assert.Equal(t, "1", FormatFloat(1))
	assert.Equal(t, "10", FormatFloat(10))
	assert.Equal(t, "0.1", FormatFloat(0.1))
	assert.Equal(t, "0.01", FormatFloat(0.01))
	assert.Equal(t, "0.001", FormatFloat(0.001))
	assert.Equal(t, "", FormatFloat(timeseries.NaN))
	assert.Equal(t, "1.2", FormatFloat(1.234))
}

func TestFormatDuration(t *testing.T) {
	d := timeseries.Duration(5 * 60) // 5 minutes
	result := FormatDuration(d, 2)
	assert.NotEmpty(t, result)

	d = timeseries.Duration(3600) // 1 hour
	result = FormatDuration(d, 2)
	assert.NotEmpty(t, result)

	d = timeseries.Duration(0)
	result = FormatDuration(d, 2)
	assert.NotEmpty(t, result)
}

func TestFormatDurationShort(t *testing.T) {
	d := timeseries.Duration(300)
	result := FormatDurationShort(d, 2)
	assert.NotEmpty(t, result)
}

func TestFormatBytes(t *testing.T) {
	val, unit := FormatBytes(0)
	assert.Equal(t, "0", val)
	assert.Equal(t, "B", unit)

	val, unit = FormatBytes(1024)
	assert.NotEmpty(t, val)

	val, unit = FormatBytes(1048576)
	assert.NotEmpty(t, val)

	val, unit = FormatBytes(float32(timeseries.NaN))
	assert.NotEmpty(t, val)
}

func TestFormatPercentage(t *testing.T) {
	assert.Equal(t, "0%", FormatPercentage(0))
	assert.Equal(t, "50%", FormatPercentage(50))
	assert.Equal(t, "99.99%", FormatPercentage(99.99))
	assert.Equal(t, "100%", FormatPercentage(100))
}

func TestUniq(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3}, Uniq([]int{1, 2, 2, 3, 3, 3}))
	assert.Equal(t, []string{"a", "b"}, Uniq([]string{"a", "b", "a"}))
	assert.Equal(t, []int{}, Uniq([]int{}))
}

func TestPtr(t *testing.T) {
	p := Ptr(42)
	assert.NotNil(t, p)
	assert.Equal(t, 42, *p)

	s := Ptr("hello")
	assert.NotNil(t, s)
	assert.Equal(t, "hello", *s)

	b := Ptr(true)
	assert.NotNil(t, b)
	assert.Equal(t, true, *b)
}

func TestStringSetMarshalJSON(t *testing.T) {
	ss := NewStringSet("a", "b", "c")
	data, err := ss.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `["a","b","c"]`, string(data))

	empty := NewStringSet()
	data, err = empty.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, `[]`, string(data))
}

func TestStringSetUnmarshalJSON(t *testing.T) {
	ss := NewStringSet()
	err := ss.UnmarshalJSON([]byte(`["x","y","z"]`))
	assert.NoError(t, err)
	assert.Equal(t, 3, ss.Len())
	assert.True(t, ss.Has("x"))
	assert.True(t, ss.Has("y"))
	assert.True(t, ss.Has("z"))

	err = ss.UnmarshalJSON([]byte(`[]`))
	assert.NoError(t, err)
	assert.Equal(t, 3, ss.Len())
}