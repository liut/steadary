package rpc

import (
	"testing"

	pb "github.com/liut/steadary/pkg/proto/sandbox"
)

// TestSandboxServiceContract 锁定 gRPC 服务描述符：
// 包名、服务名、方法名与完整方法名是稳定的线上契约，
// 不得随品牌更名而改变。
func TestSandboxServiceContract(t *testing.T) {
	desc := pb.SandboxService_ServiceDesc

	if desc.ServiceName != "sandbox.SandboxService" {
		t.Fatalf("ServiceName=%q, want %q", desc.ServiceName, "sandbox.SandboxService")
	}

	wantUnary := []string{"CloseSession", "CreateSession", "Exec", "Stats"}
	gotUnary := make(map[string]bool, len(desc.Methods))
	for _, m := range desc.Methods {
		gotUnary[m.MethodName] = true
	}
	if len(gotUnary) != len(wantUnary) {
		t.Errorf("unary methods=%d, want %d", len(gotUnary), len(wantUnary))
	}
	for _, name := range wantUnary {
		if !gotUnary[name] {
			t.Errorf("missing unary method %q", name)
		}
	}

	if len(desc.Streams) != 1 || desc.Streams[0].StreamName != "Shell" {
		t.Errorf("streams=%v, want exactly [Shell]", desc.Streams)
	}
	if !desc.Streams[0].ServerStreams || !desc.Streams[0].ClientStreams {
		t.Error("Shell must remain a bidirectional stream")
	}
}

// TestSandboxServiceFullMethodNames 锁定客户端与拦截器依赖的完整方法名。
func TestSandboxServiceFullMethodNames(t *testing.T) {
	want := map[string]string{
		"CreateSession": "/sandbox.SandboxService/CreateSession",
		"CloseSession":  "/sandbox.SandboxService/CloseSession",
		"Exec":          "/sandbox.SandboxService/Exec",
		"Shell":         "/sandbox.SandboxService/Shell",
		"Stats":         "/sandbox.SandboxService/Stats",
	}
	got := map[string]string{
		"CreateSession": pb.SandboxService_CreateSession_FullMethodName,
		"CloseSession":  pb.SandboxService_CloseSession_FullMethodName,
		"Exec":          pb.SandboxService_Exec_FullMethodName,
		"Shell":         pb.SandboxService_Shell_FullMethodName,
		"Stats":         pb.SandboxService_Stats_FullMethodName,
	}
	for name, wantName := range want {
		if got[name] != wantName {
			t.Errorf("full method name for %s = %q, want %q", name, got[name], wantName)
		}
	}
}
