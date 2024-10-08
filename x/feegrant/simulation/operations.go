package simulation

import (
	"math/rand"

	"cosmossdk.io/x/feegrant"
	"cosmossdk.io/x/feegrant/keeper"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
)

// Simulation operation weights constants
const (
	OpWeightMsgGrantAllowance        = "op_weight_msg_grant_fee_allowance"
	OpWeightMsgRevokeAllowance       = "op_weight_msg_grant_revoke_allowance"
	DefaultWeightGrantAllowance  int = 100
	DefaultWeightRevokeAllowance int = 100
)

var (
	TypeMsgGrantAllowance  = sdk.MsgTypeURL(&feegrant.MsgGrantAllowance{})
	TypeMsgRevokeAllowance = sdk.MsgTypeURL(&feegrant.MsgRevokeAllowance{})
)

func WeightedOperations(
	appParams simtypes.AppParams,
	txConfig client.TxConfig,
	ak feegrant.AccountKeeper,
	bk feegrant.BankKeeper,
	k keeper.Keeper,
) simulation.WeightedOperations {
	var (
		weightMsgGrantAllowance  int
		weightMsgRevokeAllowance int
	)

	appParams.GetOrGenerate(OpWeightMsgGrantAllowance, &weightMsgGrantAllowance, nil,
		func(_ *rand.Rand) {
			weightMsgGrantAllowance = DefaultWeightGrantAllowance
		},
	)

	appParams.GetOrGenerate(OpWeightMsgRevokeAllowance, &weightMsgRevokeAllowance, nil,
		func(_ *rand.Rand) {
			weightMsgRevokeAllowance = DefaultWeightRevokeAllowance
		},
	)

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(
			weightMsgGrantAllowance,
			SimulateMsgGrantAllowance(txConfig, ak, bk, k),
		),
		simulation.NewWeightedOperation(
			weightMsgRevokeAllowance,
			SimulateMsgRevokeAllowance(txConfig, ak, bk, k),
		),
	}
}

// SimulateMsgGrantAllowance generates MsgGrantAllowance with random values.
func SimulateMsgGrantAllowance(
	txConfig client.TxConfig,
	ak feegrant.AccountKeeper,
	bk feegrant.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app simtypes.AppEntrypoint, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		granter, _ := simtypes.RandomAcc(r, accs)
		grantee, _ := simtypes.RandomAcc(r, accs)
		granterStr, err := ak.AddressCodec().BytesToString(granter.Address)
		if err != nil {
			return simtypes.OperationMsg{}, nil, err
		}
		granteeStr, err := ak.AddressCodec().BytesToString(grantee.Address)
		if err != nil {
			return simtypes.OperationMsg{}, nil, err
		}

		if granteeStr == granterStr {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgGrantAllowance, "grantee and granter cannot be same"), nil, nil
		}

		if f, _ := k.GetAllowance(ctx, granter.Address, grantee.Address); f != nil {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgGrantAllowance, "fee allowance exists"), nil, nil
		}

		account := ak.GetAccount(ctx, granter.Address)

		spendableCoins := bk.SpendableCoins(ctx, account.GetAddress())
		if spendableCoins.Empty() {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgGrantAllowance, "unable to grant empty coins as SpendLimit"), nil, nil
		}

		oneYear := ctx.HeaderInfo().Time.AddDate(1, 0, 0)
		msg, err := feegrant.NewMsgGrantAllowance(&feegrant.BasicAllowance{
			SpendLimit: spendableCoins,
			Expiration: &oneYear,
		}, granterStr, granteeStr)
		if err != nil {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgGrantAllowance, err.Error()), nil, err
		}

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txConfig,
			Cdc:             nil,
			Msg:             msg,
			Context:         ctx,
			SimAccount:      granter,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      feegrant.ModuleName,
			CoinsSpentInMsg: spendableCoins,
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgRevokeAllowance generates a MsgRevokeAllowance with random values.
func SimulateMsgRevokeAllowance(
	txConfig client.TxConfig,
	ak feegrant.AccountKeeper,
	bk feegrant.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app simtypes.AppEntrypoint, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		hasGrant := false

		var granterAddr sdk.AccAddress
		var granteeAddr sdk.AccAddress
		err := k.IterateAllFeeAllowances(ctx, func(grant feegrant.Grant) bool {
			granter, err := ak.AddressCodec().StringToBytes(grant.Granter)
			if err != nil {
				panic(err)
			}
			grantee, err := ak.AddressCodec().StringToBytes(grant.Grantee)
			if err != nil {
				panic(err)
			}
			granterAddr = granter
			granteeAddr = grantee
			hasGrant = true
			return true
		})
		if err != nil {
			return simtypes.OperationMsg{}, nil, err
		}

		if !hasGrant {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgRevokeAllowance, "no grants"), nil, nil
		}
		granter, ok := simtypes.FindAccount(accs, granterAddr)

		if !ok {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgRevokeAllowance, "Account not found"), nil, nil
		}

		account := ak.GetAccount(ctx, granter.Address)
		spendableCoins := bk.SpendableCoins(ctx, account.GetAddress())

		granterStr, err := ak.AddressCodec().BytesToString(granterAddr)
		if err != nil {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgRevokeAllowance, err.Error()), nil, err
		}
		granteeStr, err := ak.AddressCodec().BytesToString(granteeAddr)
		if err != nil {
			return simtypes.NoOpMsg(feegrant.ModuleName, TypeMsgRevokeAllowance, err.Error()), nil, err
		}
		msg := feegrant.NewMsgRevokeAllowance(granterStr, granteeStr)

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txConfig,
			Cdc:             nil,
			Msg:             &msg,
			Context:         ctx,
			SimAccount:      granter,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      feegrant.ModuleName,
			CoinsSpentInMsg: spendableCoins,
		}

		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}
