package manifest

import "testing"

func TestDeleteOrder_ReversesApplyOrder(t *testing.T) {
	apply := ApplyOrder()
	del := DeleteOrder()

	if len(del) != len(apply) {
		t.Fatalf("DeleteOrder length %d != ApplyOrder length %d", len(del), len(apply))
	}

	for i := range apply {
		want := apply[len(apply)-1-i]
		if del[i] != want {
			t.Errorf("DeleteOrder[%d] = %s, want %s", i, del[i], want)
		}
	}
}

func TestDeleteOrder_FirstElementIsLastOfApplyOrder(t *testing.T) {
	apply := ApplyOrder()
	del := DeleteOrder()

	if del[0] != apply[len(apply)-1] {
		t.Errorf("DeleteOrder[0] = %s, want %s (last of ApplyOrder)", del[0], apply[len(apply)-1])
	}
}

func TestDeleteOrder_LastElementIsFirstOfApplyOrder(t *testing.T) {
	apply := ApplyOrder()
	del := DeleteOrder()

	last := del[len(del)-1]
	if last != apply[0] {
		t.Errorf("DeleteOrder last = %s, want %s (first of ApplyOrder)", last, apply[0])
	}
}
