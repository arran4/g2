package g2

import (
	"fmt"
	"reflect"
	"testing"
)

type mockPortageContext struct {
	responses map[string]map[string]string
	calls     []string
}

func (m *mockPortageContext) QueryMetadata(category, pkg, pvr string, keys []string) (map[string]string, error) {
	key := category + "/" + pkg + "-" + pvr
	m.calls = append(m.calls, key)
	if resp, ok := m.responses[key]; ok {
		result := make(map[string]string)
		for _, k := range keys {
			if val, exists := resp[k]; exists {
				result[k] = val
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("unexpected package query %q", key)
}

// OSExecPortageContext uses os/exec to run portageq metadata. Test that it compiles by assigning it.
var _ PortageContext = (*OSExecPortageContext)(nil)

func TestMockPortageContext(t *testing.T) {
	mock := &mockPortageContext{
		responses: map[string]map[string]string{
			"sys-apps/test-1.0": {
				"BDEPEND": "dev-build/cmake",
			},
		},
	}

	res, err := mock.QueryMetadata("sys-apps", "test", "1.0", []string{"BDEPEND"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res, map[string]string{"BDEPEND": "dev-build/cmake"}) {
		t.Fatalf("Unexpected response: %v", res)
	}
}
