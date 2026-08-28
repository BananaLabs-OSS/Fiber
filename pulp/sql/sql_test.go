package sql

import "testing"

func TestFailedTransactionCloseClearsLocalState(t *testing.T) {
	for _, closeTx := range []struct {
		name string
		fn   func(*Tx) error
	}{
		{name: "commit", fn: (*Tx).Commit},
		{name: "rollback", fn: (*Tx).Rollback},
	} {
		t.Run(closeTx.name, func(t *testing.T) {
			conn := &Conn{inTx: true}
			if err := closeTx.fn(&Tx{conn: conn}); err == nil {
				t.Fatal("unbound native capability unexpectedly succeeded")
			}
			if conn.inTx {
				t.Fatal("failed transaction close left connection marked in-transaction")
			}
		})
	}
}
