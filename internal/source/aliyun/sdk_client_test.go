package aliyun

import (
	"reflect"
	"testing"
	"time"
)

func TestApplyRequestTimeoutSetsReadAndConnectTimeout(t *testing.T) {
	request := &fakeTimeoutRequest{}

	applyRequestTimeout(request, 15*time.Second)

	if request.readTimeout != 15*time.Second {
		t.Fatalf("read timeout = %s, want 15s", request.readTimeout)
	}
	if request.connectTimeout != 15*time.Second {
		t.Fatalf("connect timeout = %s, want 15s", request.connectTimeout)
	}
}

func TestListTagResourcesRequestsIncludeResourceIDs(t *testing.T) {
	resourceIDs := []string{"instance-a", "instance-b"}

	rdsRequest := newRDSListTagResourcesRequest(resourceIDs, "next-token", 15*time.Second)
	if rdsRequest.ResourceType != aliyunTagResourceType {
		t.Fatalf("rds resource type = %q, want %q", rdsRequest.ResourceType, aliyunTagResourceType)
	}
	if rdsRequest.NextToken != "next-token" {
		t.Fatalf("rds next token = %q", rdsRequest.NextToken)
	}
	assertResourceIDs(t, "rds", rdsRequest.ResourceId, resourceIDs)

	mongoRequest := newMongoListTagResourcesRequest(resourceIDs, "next-token", 15*time.Second)
	if mongoRequest.ResourceType != aliyunTagResourceType {
		t.Fatalf("mongo resource type = %q, want %q", mongoRequest.ResourceType, aliyunTagResourceType)
	}
	if mongoRequest.NextToken != "next-token" {
		t.Fatalf("mongo next token = %q", mongoRequest.NextToken)
	}
	assertResourceIDs(t, "mongo", mongoRequest.ResourceId, resourceIDs)
}

func TestNewECSDescribeInstancesRequestDoesNotFilterStatus(t *testing.T) {
	request := newECSDescribeInstancesRequest(1, 15*time.Second)

	if request.Status != "" {
		t.Fatalf("ecs status filter = %q, want empty", request.Status)
	}
}

func assertResourceIDs(t *testing.T, name string, got *[]string, want []string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s resource ids = nil", name)
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("%s resource ids = %#v, want %#v", name, *got, want)
	}
}

type fakeTimeoutRequest struct {
	readTimeout    time.Duration
	connectTimeout time.Duration
}

func (r *fakeTimeoutRequest) SetReadTimeout(timeout time.Duration) {
	r.readTimeout = timeout
}

func (r *fakeTimeoutRequest) SetConnectTimeout(timeout time.Duration) {
	r.connectTimeout = timeout
}
