package qgb_test

import (
	"testing"

	"github.com/celestiaorg/celestia-app/x/qgb/types"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/teststaking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/celestiaorg/celestia-app/testutil"
	"github.com/celestiaorg/celestia-app/x/qgb"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstAttestationIsValset(t *testing.T) {
	input, ctx := testutil.SetupFiveValChain(t)
	pk := input.QgbKeeper

	// EndBlocker should set a new validator set
	qgb.EndBlocker(ctx, *pk)

	require.Equal(t, uint64(1), pk.GetLatestAttestationNonce(ctx))
	attestation, found, err := pk.GetAttestationByNonce(ctx, 1)
	require.Nil(t, err)
	require.True(t, found)
	require.NotNil(t, attestation)
	require.Equal(t, uint64(1), attestation.GetNonce())

	// get the valset
	require.Equal(t, types.ValsetRequestType, attestation.Type())
	vs, ok := attestation.(*types.Valset)
	require.True(t, ok)
	require.NotNil(t, vs)
}

func TestValsetCreationWhenValidatorUnbonds(t *testing.T) {
	input, ctx := testutil.SetupFiveValChain(t)
	pk := input.QgbKeeper

	// run abci methods after chain init
	staking.EndBlocker(input.Context, input.StakingKeeper)
	qgb.EndBlocker(ctx, *pk)

	// current attestation expectedNonce should be 1 because a valset has been emitted upon chain init.
	currentAttestationNonce := pk.GetLatestAttestationNonce(ctx)
	require.Equal(t, uint64(1), currentAttestationNonce)

	input.Context = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	msgServer := stakingkeeper.NewMsgServerImpl(input.StakingKeeper)

	undelegateMsg := testutil.NewTestMsgUnDelegateValidator(testutil.ValAddrs[0], testutil.StakingAmount)
	_, err := msgServer.Undelegate(input.Context, undelegateMsg)
	require.NoError(t, err)
	staking.EndBlocker(input.Context, input.StakingKeeper)
	qgb.EndBlocker(input.Context, *pk)
	input.Context = ctx.WithBlockHeight(ctx.BlockHeight() + 10)

	assert.Equal(t, currentAttestationNonce+1, pk.GetLatestAttestationNonce(ctx))
}

func TestEndBlockerAfterEditingOrchestratorAddress(t *testing.T) {
	input, ctx := testutil.SetupFiveValChain(t)
	pk := input.QgbKeeper

	// run abci methods after chain init
	staking.EndBlocker(input.Context, input.StakingKeeper)
	qgb.EndBlocker(ctx, *pk)

	// current attestation expectedNonce should be 1 because a valset has been emitted upon chain init.
	currentAttestationNonce := pk.GetLatestAttestationNonce(ctx)
	require.Equal(t, uint64(1), currentAttestationNonce)

	input.Context = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	msgServer := stakingkeeper.NewMsgServerImpl(input.StakingKeeper)

	newOrchAddr := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())
	editMsg := stakingtypes.NewMsgEditValidator(
		testutil.ValAddrs[1],
		stakingtypes.Description{},
		nil,
		nil,
		&newOrchAddr,
		nil,
	)
	_, err := msgServer.EditValidator(input.Context, editMsg)
	require.NoError(t, err)
	staking.EndBlocker(input.Context, input.StakingKeeper)
	qgb.EndBlocker(input.Context, *pk)
	input.Context = ctx.WithBlockHeight(ctx.BlockHeight() + 10)

	// the nonce shouldn't increase because a change in the orchestrator address shouldn't
	// cause a change in the validator set, from the QGB perspective.
	// check x/qgb/types/validator.go for more details.
	assert.Equal(t, currentAttestationNonce, pk.GetLatestAttestationNonce(ctx))
}

func TestValsetCreationWhenEditingEVMAddr(t *testing.T) {
	input, ctx := testutil.SetupFiveValChain(t)
	pk := input.QgbKeeper

	// run abci methods after chain init
	staking.EndBlocker(input.Context, input.StakingKeeper)
	qgb.EndBlocker(ctx, *pk)

	// current attestation expectedNonce should be 1 because a valset has been emitted upon chain init.
	currentAttestationNonce := pk.GetLatestAttestationNonce(ctx)
	require.Equal(t, uint64(1), currentAttestationNonce)

	input.Context = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
	msgServer := stakingkeeper.NewMsgServerImpl(input.StakingKeeper)

	newEVMAddr, err := teststaking.RandomEVMAddress()
	require.NoError(t, err)
	editMsg := stakingtypes.NewMsgEditValidator(
		testutil.ValAddrs[1],
		stakingtypes.Description{},
		nil,
		nil,
		nil,
		newEVMAddr,
	)
	_, err = msgServer.EditValidator(input.Context, editMsg)
	require.NoError(t, err)
	staking.EndBlocker(input.Context, input.StakingKeeper)
	qgb.EndBlocker(input.Context, *pk)
	input.Context = ctx.WithBlockHeight(ctx.BlockHeight() + 10)

	assert.Equal(t, currentAttestationNonce+1, pk.GetLatestAttestationNonce(ctx))
}

func TestSetValset(t *testing.T) {
	input, ctx := testutil.SetupFiveValChain(t)
	pk := input.QgbKeeper

	vs, err := pk.GetCurrentValset(ctx)
	require.Nil(t, err)
	err = pk.SetAttestationRequest(ctx, &vs)
	require.Nil(t, err)

	require.Equal(t, uint64(1), pk.GetLatestAttestationNonce(ctx))
}

func TestSetDataCommitment(t *testing.T) {
	input, ctx := testutil.SetupFiveValChain(t)
	qk := input.QgbKeeper

	input.Context = ctx.WithBlockHeight(int64(qk.GetDataCommitmentWindowParam(ctx)))
	vs, err := qk.GetCurrentDataCommitment(ctx)
	require.Nil(t, err)
	err = qk.SetAttestationRequest(ctx, &vs)
	require.Nil(t, err)

	require.Equal(t, uint64(1), qk.GetLatestAttestationNonce(ctx))
}

func TestDataCommitmentCreation(t *testing.T) {
	input, ctx := testutil.SetupFiveValChain(t)
	qk := input.QgbKeeper

	// run abci methods after chain init
	staking.EndBlocker(input.Context, input.StakingKeeper)
	qgb.EndBlocker(ctx, *qk)

	// current attestation nonce should be 1 because a valset has been emitted upon chain init.
	currentAttestationNonce := qk.GetLatestAttestationNonce(ctx)
	require.Equal(t, uint64(1), currentAttestationNonce)

	// increment height to be the same as the data commitment window
	newHeight := int64(qk.GetDataCommitmentWindowParam(ctx))
	input.Context = ctx.WithBlockHeight(newHeight)
	qgb.EndBlocker(input.Context, *qk)

	require.Less(t, newHeight, ctx.BlockHeight())
	assert.Equal(t, uint64(2), qk.GetLatestAttestationNonce(ctx))
}
