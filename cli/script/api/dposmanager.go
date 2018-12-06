package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	account2 "github.com/elastos/Elastos.ELA/account"
	"github.com/elastos/Elastos.ELA/common"
	"github.com/elastos/Elastos.ELA/common/config"
	"github.com/elastos/Elastos.ELA/crypto"
	"github.com/elastos/Elastos.ELA/dpos/account"
	"github.com/elastos/Elastos.ELA/dpos/log"
	. "github.com/elastos/Elastos.ELA/dpos/manager"

	"github.com/yuin/gopher-lua"
)

const (
	luaDposManagerTypeName = "dpos_manager"

	luaConsensusIsOnDutyName  = "IsOnDuty"
	luaConsensusIsReadyName   = "IsReady"
	luaConsensusIsRunningName = "IsRunning"
)

func RegisterDposManagerType(L *lua.LState) {
	mt := L.NewTypeMetatable(luaDposManagerTypeName)
	L.SetGlobal("dpos_manager", mt)
	// static attributes
	L.SetField(mt, "new", L.NewFunction(newDposManager))
	// methods
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), dposManagerMethods))
}

// Constructor
func newDposManager(L *lua.LState) int {
	n := checkDposNetwork(L, 1)
	if n == nil {
		fmt.Println("Network nil, create manager error.")
		return 0
	}
	a := checkArbitrators(L, 2)
	index := uint32(L.ToInt(3))
	if index >= 5 {
		L.ArgError(1, "Index invalid.")
		return 0
	}

	dposManager := NewManager(arbitratorsPublicKeys[index], a)
	mockManager := &manager{
		DposManager: dposManager,
	}

	priKey, _ := common.HexStringToBytes(arbitratorsPrivateKeys[index])
	pub, _ := common.HexStringToBytes(arbitratorsPublicKeys[index])
	pubKey, _ := crypto.DecodePoint(pub)
	mockManager.Account = account.NewDposAccountFromExisting(&account2.Account{
		PrivateKey: priKey,
		PublicKey:  pubKey,
	})

	mockManager.EventMonitor = log.NewEventMoniter()
	mockManager.EventMonitor.RegisterListener(&log.EventLogs{})

	mockManager.Handler = NewHandler(n, dposManager, mockManager.EventMonitor)

	mockManager.Consensus = NewConsensus(dposManager, time.Duration(config.Parameters.ArbiterConfiguration.SignTolerance)*time.Second, mockManager.Handler)
	mockManager.Dispatcher, mockManager.IllegalMonitor = NewDispatcherAndIllegalMonitor(mockManager.Consensus, mockManager.EventMonitor, n, dposManager, mockManager.Account)
	mockManager.Handler.Initialize(mockManager.Dispatcher, mockManager.Consensus)

	dposManager.Initialize(mockManager.Handler, mockManager.Dispatcher, mockManager.Consensus, n, mockManager.IllegalMonitor)
	n.Initialize(mockManager.Dispatcher)
	n.SetListener(dposManager)

	ud := L.NewUserData()
	ud.Value = mockManager
	L.SetMetatable(ud, L.GetTypeMetatable(luaDposManagerTypeName))
	L.Push(ud)

	return 1
}

// Checks whether the first lua argument is a *LUserData with *Attribute and returns this *Attribute.
func checkDposManager(L *lua.LState, idx int) *manager {
	ud := L.CheckUserData(idx)
	if v, ok := ud.Value.(*manager); ok {
		return v
	}
	L.ArgError(1, "DPosManager expected")
	return nil
}

var dposManagerMethods = map[string]lua.LGFunction{
	"public_key":     dposManagerPublicKey,
	"dump_consensus": dposManagerDumpConsensus,

	"is_on_duty":        dposManagerCheckOnDuty,
	"is_status_ready":   dposManagerCheckStatusReady,
	"is_status_running": dposManagerCheckStatusRunning,

	"sign_proposal": dposManagerSignProposal,
	"sign_vote":     dposManagerSignVote,
}

func dposManagerPublicKey(L *lua.LState) int {
	m := checkDposManager(L, 1)
	L.Push(lua.LString(m.GetPublicKey()))

	return 1
}

func dposManagerDumpConsensus(L *lua.LState) int {
	m := checkDposManager(L, 1)
	consensus := m.Consensus

	content := map[string]string{
		luaConsensusIsOnDutyName:  strconv.FormatBool(consensus.IsOnDuty()),
		luaConsensusIsReadyName:   strconv.FormatBool(consensus.IsReady()),
		luaConsensusIsRunningName: strconv.FormatBool(consensus.IsRunning()),
	}
	jsonStr, _ := json.Marshal(content)
	L.Push(lua.LString(jsonStr))

	return 1
}

func dposManagerCheckOnDuty(L *lua.LState) int {
	m := checkDposManager(L, 1)
	L.Push(lua.LBool(m.Consensus.IsOnDuty()))
	return 1
}

func dposManagerCheckStatusReady(L *lua.LState) int {
	m := checkDposManager(L, 1)
	L.Push(lua.LBool(m.Consensus.IsReady()))
	return 1
}

func dposManagerCheckStatusRunning(L *lua.LState) int {
	m := checkDposManager(L, 1)
	L.Push(lua.LBool(m.Consensus.IsRunning()))
	return 1
}

func dposManagerSignProposal(L *lua.LState) int {
	m := checkDposManager(L, 1)
	p := checkProposal(L, 2)

	result := false
	if sign, err := m.Account.SignProposal(p); err == nil {
		p.Sign = sign
		result = true
	}
	L.Push(lua.LBool(result))

	return 1
}

func dposManagerSignVote(L *lua.LState) int {
	m := checkDposManager(L, 1)
	v := checkVote(L, 2)

	result := false
	if sign, err := m.Account.SignVote(v); err == nil {
		v.Sign = sign
		result = true
	}
	L.Push(lua.LBool(result))

	return 1
}

//mock object of dpos manager
type manager struct {
	DposManager
	Account        account.DposAccount
	Consensus      Consensus
	EventMonitor   *log.EventMonitor
	Handler        DposHandlerSwitch
	Dispatcher     ProposalDispatcher
	IllegalMonitor IllegalBehaviorMonitor
}