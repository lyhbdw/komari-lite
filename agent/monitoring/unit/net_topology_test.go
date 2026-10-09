package monitoring

import (
	"testing"
	"time"
)

func TestCalcNicEpoch(t *testing.T) {
	bootID1 := "7e8febc1-aec6-4e31-adfd-c02065fcbc14"
	bootID2 := "b0071337-0000-0000-0000-000000000000"

	nicsA := []string{"eth0", "eth1"}
	nicsB := []string{"eth1", "eth0"} // 顺序颠倒但集合一致
	nicsC := []string{"eth0"}          // 集合变更

	epoch1 := calcNicEpoch(bootID1, nicsA)
	epoch2 := calcNicEpoch(bootID1, nicsB)
	epoch3 := calcNicEpoch(bootID1, nicsC)
	epoch4 := calcNicEpoch(bootID2, nicsA)

	if epoch1 != epoch2 {
		t.Fatalf("expected epoch to be invariant to nic order: %s vs %s", epoch1, epoch2)
	}
	if epoch1 == epoch3 {
		t.Fatalf("expected epoch to change when nics set changes: %s vs %s", epoch1, epoch3)
	}
	if epoch1 == epoch4 {
		t.Fatalf("expected epoch to change when bootID changes: %s vs %s", epoch1, epoch4)
	}
}

func TestUpdateNetworkSpeedSampleRebaselineOnEpochChange(t *testing.T) {
	now := time.Now()
	epoch1 := "boot-1/1234"
	epoch2 := "boot-2/5678" // 模拟系统重启或网卡集合变更

	// 首次采样，建立 baseline，返回 0, 0
	up, down := updateNetworkSpeedSample(1000, 2000, epoch1, now)
	if up != 0 || down != 0 {
		t.Fatalf("expected initial rate to be 0, got up=%d down=%d", up, down)
	}

	// 1秒后采样，同一 epoch，正常计算速率
	up, down = updateNetworkSpeedSample(2000, 4000, epoch1, now.Add(1*time.Second))
	if up != 1000 || down != 2000 {
		t.Fatalf("expected rate up=1000 down=2000, got up=%d down=%d", up, down)
	}

	// 发生重启 (epoch 变化)，即使当前计数器比之前小或者大，必须触发 Re-baseline 返回 0, 0，不发生异常洪峰
	up, down = updateNetworkSpeedSample(500, 1000, epoch2, now.Add(2*time.Second))
	if up != 0 || down != 0 {
		t.Fatalf("expected re-baseline on epoch change to return 0, got up=%d down=%d", up, down)
	}

	// 再次在 epoch2 下正常递增
	up, down = updateNetworkSpeedSample(800, 1600, epoch2, now.Add(3*time.Second))
	if up != 300 || down != 600 {
		t.Fatalf("expected rate up=300 down=600 after re-baseline, got up=%d down=%d", up, down)
	}
}
