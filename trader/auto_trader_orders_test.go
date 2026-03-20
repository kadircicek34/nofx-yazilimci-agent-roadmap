package trader

import (
	"errors"
	"testing"
	"time"
)

type protectionTestTrader struct {
	stopLossErr error
	tpErr       error

	stopLossCalled    bool
	takeProfitCalled  bool
	cancelStopsCalled bool
	closeLongCalled   bool
	closeShortCalled  bool
}

func (t *protectionTestTrader) GetBalance() (map[string]interface{}, error)     { return nil, nil }
func (t *protectionTestTrader) GetPositions() ([]map[string]interface{}, error) { return nil, nil }
func (t *protectionTestTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": "open-long"}, nil
}
func (t *protectionTestTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return map[string]interface{}{"orderId": "open-short"}, nil
}
func (t *protectionTestTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	t.closeLongCalled = true
	return map[string]interface{}{"orderId": "close-long"}, nil
}
func (t *protectionTestTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	t.closeShortCalled = true
	return map[string]interface{}{"orderId": "close-short"}, nil
}
func (t *protectionTestTrader) SetLeverage(symbol string, leverage int) error         { return nil }
func (t *protectionTestTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (t *protectionTestTrader) GetMarketPrice(symbol string) (float64, error)         { return 0, nil }
func (t *protectionTestTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	t.stopLossCalled = true
	return t.stopLossErr
}
func (t *protectionTestTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	t.takeProfitCalled = true
	return t.tpErr
}
func (t *protectionTestTrader) CancelStopLossOrders(symbol string) error   { return nil }
func (t *protectionTestTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (t *protectionTestTrader) CancelAllOrders(symbol string) error        { return nil }
func (t *protectionTestTrader) CancelStopOrders(symbol string) error {
	t.cancelStopsCalled = true
	return nil
}
func (t *protectionTestTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "1", nil
}
func (t *protectionTestTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}
func (t *protectionTestTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return nil, nil
}
func (t *protectionTestTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) { return nil, nil }

func TestEnsureProtectionOrders_RollsBackOnStopLossFailure(t *testing.T) {
	stub := &protectionTestTrader{stopLossErr: errors.New("stop loss failed")}
	at := &AutoTrader{trader: stub}

	err := at.ensureProtectionOrders("BTCUSDT", "LONG", "long", 1, 90000, 110000)
	if err == nil {
		t.Fatal("expected error when stop loss placement fails")
	}
	if !stub.stopLossCalled {
		t.Fatal("expected stop loss to be attempted")
	}
	if !stub.cancelStopsCalled {
		t.Fatal("expected existing stop orders to be cancelled during rollback")
	}
	if !stub.closeLongCalled {
		t.Fatal("expected emergency close long to be called")
	}
}

func TestEnsureProtectionOrders_RollsBackOnTakeProfitFailure(t *testing.T) {
	stub := &protectionTestTrader{tpErr: errors.New("take profit failed")}
	at := &AutoTrader{trader: stub}

	err := at.ensureProtectionOrders("ETHUSDT", "SHORT", "short", 1, 3500, 3100)
	if err == nil {
		t.Fatal("expected error when take profit placement fails")
	}
	if !stub.stopLossCalled || !stub.takeProfitCalled {
		t.Fatal("expected both stop loss and take profit placement attempts")
	}
	if !stub.cancelStopsCalled {
		t.Fatal("expected stop orders cleanup on rollback")
	}
	if !stub.closeShortCalled {
		t.Fatal("expected emergency close short to be called")
	}
}
