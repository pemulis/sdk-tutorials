package scavenge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/okwme/scavenge/x/scavenge/internal/types"
	"github.com/tendermint/tendermint/crypto"
)

// NewHandler creates an sdk.Handler for all the scavenge type messages
func NewHandler(k Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		switch msg := msg.(type) {
		case MsgCreateScavenge:
			return handleMsgCreateScavenge(ctx, k, msg)
		case MsgCommitSolution:
			return handleMsgCommitSolution(ctx, k, msg)
		case MsgRevealSolution:
			return handleMsgRevealSolution(ctx, k, msg)
		default:
			errMsg := fmt.Sprintf("unrecognized %s message type: %T", types.ModuleName, msg)
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

// handleMsgCreateScavenge creates a new scavenge and moves the reward into escrow
func handleMsgCreateScavenge(ctx sdk.Context, k Keeper, msg MsgCreateScavenge) sdk.Result {
	var scavenge = types.Scavenge{
		Creator:      msg.Creator,
		Description:  msg.Description,
		SolutionHash: msg.SolutionHash,
		Reward:       msg.Reward,
	}
	_, err := k.GetScavenge(ctx, scavenge.SolutionHash)
	if err == nil {
		return sdk.NewError(types.DefaultCodespace, types.CodeInvalid, "Scavenge with that solution hash already exists").Result()
	}
	moduleAcct := sdk.AccAddress(crypto.AddressHash([]byte(types.ModuleName)))
	sdkError := k.CoinKeeper.SendCoins(ctx, scavenge.Creator, moduleAcct, scavenge.Reward)
	if sdkError != nil {
		return sdkError.Result()
	}
	k.SetScavenge(ctx, scavenge)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeyAction, types.EventTypeCreateScavenge),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Creator.String()),
			sdk.NewAttribute(types.AttributeDescription, msg.Description),
			sdk.NewAttribute(types.AttributeSolutionHash, msg.SolutionHash),
			sdk.NewAttribute(types.AttributeReward, msg.Reward.String()),
		),
	)
	return sdk.Result{Events: ctx.EventManager().Events()}
}

func handleMsgCommitSolution(ctx sdk.Context, k Keeper, msg MsgCommitSolution) sdk.Result {
	var commit = types.Commit{
		Scavenger:             msg.Scavenger,
		SolutionHash:          msg.SolutionHash,
		SolutionScavengerHash: msg.SolutionScavengerHash,
	}
	_, err := k.GetCommit(ctx, commit.SolutionScavengerHash)
	if err == nil {
		return sdk.NewError(types.DefaultCodespace, types.CodeInvalid, "Commit with that hash already exists").Result()
	}
	k.SetCommit(ctx, commit)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeyAction, types.EventTypeCommitSolution),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Scavenger.String()),
			sdk.NewAttribute(types.AttributeSolutionHash, msg.SolutionHash),
			sdk.NewAttribute(types.AttributeSolutionScavengerHash, msg.SolutionScavengerHash),
		),
	)
	return sdk.Result{Events: ctx.EventManager().Events()}
}

func handleMsgRevealSolution(ctx sdk.Context, k Keeper, msg MsgRevealSolution) sdk.Result {
	var solutionScavengerBytes = []byte(msg.Solution + msg.Scavenger.String())
	var solutionScavengerHash = sha256.Sum256(solutionScavengerBytes)
	var solutionScavengerHashString = hex.EncodeToString(solutionScavengerHash[:])
	_, err := k.GetCommit(ctx, solutionScavengerHashString)
	if err != nil {
		return sdk.NewError(types.DefaultCodespace, types.CodeInvalid, "Commit with that hash doesn't exists").Result()
	}

	var solutionHash = sha256.Sum256([]byte(msg.Solution))
	var solutionHashString = hex.EncodeToString(solutionHash[:])
	var scavenge types.Scavenge
	scavenge, err = k.GetScavenge(ctx, solutionHashString)
	if err != nil {
		return sdk.NewError(types.DefaultCodespace, types.CodeInvalid, "Scavenge with that solution hash doesn't exists").Result()
	}
	if scavenge.Scavenger != nil {
		return sdk.NewError(types.DefaultCodespace, types.CodeInvalid, "Scavenge has already been solved").Result()
	}
	scavenge.Scavenger = msg.Scavenger
	scavenge.Solution = msg.Solution

	moduleAcct := sdk.AccAddress(crypto.AddressHash([]byte(types.ModuleName)))
	sdkError := k.CoinKeeper.SendCoins(ctx, moduleAcct, scavenge.Scavenger, scavenge.Reward)
	if sdkError != nil {
		return sdkError.Result()
	}
	k.SetScavenge(ctx, scavenge)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeyAction, types.EventTypeSolveScavenge),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Scavenger.String()),
			sdk.NewAttribute(types.AttributeSolutionHash, solutionHashString),
			sdk.NewAttribute(types.AttributeDescription, scavenge.Description),
			sdk.NewAttribute(types.AttributeSolution, msg.Solution),
			sdk.NewAttribute(types.AttributeScavenger, scavenge.Scavenger.String()),
			sdk.NewAttribute(types.AttributeReward, scavenge.Reward.String()),
		),
	)
	return sdk.Result{Events: ctx.EventManager().Events()}
}
