package da

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	daproto "github.com/celestiaorg/celestia-app/proto/da"
	"github.com/celestiaorg/rsmt2d"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/types"
)

const (
	maxExtendedSquareWidth = appconsts.DefaultMaxSquareSize * 2
	minExtendedSquareWidth = appconsts.DefaultMinSquareSize * 2
)

// DataAvailabilityHeader (DAHeader) contains the row and column roots of the
// erasure coded version of the data in Block.Data. The original Block.Data is
// split into shares and arranged in a square of width squareSize. Then, this
// square is "extended" into an extended data square (EDS) of width 2*squareSize
// by applying Reed-Solomon encoding. For details see Section 5.2 of
// https://arxiv.org/abs/1809.09044 or the Celestia specification:
// https://github.com/celestiaorg/celestia-specs/blob/master/src/specs/data_structures.md#availabledataheader
type DataAvailabilityHeader struct {
	// RowRoot_j = root((M_{j,1} || M_{j,2} || ... || M_{j,2k} ))
	RowsRoots [][]byte `json:"row_roots"`
	// ColumnRoot_j = root((M_{1,j} || M_{2,j} || ... || M_{2k,j} ))
	ColumnRoots [][]byte `json:"column_roots"`
	// hash is the Merkle root of the row and column roots. This field is the
	// memoized result from `Hash()`.
	hash []byte
}

// NewDataAvailabilityHeader generates a DataAvailability header using the provided square size and shares
func NewDataAvailabilityHeader(eds *rsmt2d.ExtendedDataSquare) DataAvailabilityHeader {
	// generate the row and col roots using the EDS
	dah := DataAvailabilityHeader{
		RowsRoots:   eds.RowRoots(),
		ColumnRoots: eds.ColRoots(),
	}

	// generate the hash of the data using the new roots
	dah.Hash()

	return dah
}

func ExtendShares(squareSize uint64, shares [][]byte) (*rsmt2d.ExtendedDataSquare, error) {
	// Check that square size is with range
	if squareSize < appconsts.DefaultMinSquareSize || squareSize > appconsts.DefaultMaxSquareSize {
		return nil, fmt.Errorf(
			"invalid square size: min %d max %d provided %d",
			appconsts.DefaultMinSquareSize,
			appconsts.DefaultMaxSquareSize,
			squareSize,
		)
	}
	// check that valid number of shares have been provided
	if squareSize*squareSize != uint64(len(shares)) {
		return nil, fmt.Errorf(
			"must provide valid number of shares for square size: got %d wanted %d",
			len(shares),
			squareSize*squareSize,
		)
	}
	// here we construct a tree
	return rsmt2d.ComputeExtendedDataSquare(shares, appconsts.DefaultCodec(), wrapper.NewConstructor(squareSize))
}

// String returns hex representation of merkle hash of the DAHeader.
func (dah *DataAvailabilityHeader) String() string {
	if dah == nil {
		return "<nil DAHeader>"
	}
	return fmt.Sprintf("%X", dah.Hash())
}

// Equals checks equality of two DAHeaders.
func (dah *DataAvailabilityHeader) Equals(to *DataAvailabilityHeader) bool {
	return bytes.Equal(dah.Hash(), to.Hash())
}

// Hash computes the Merkle root of the row and column roots. Hash memoizes the
// result in `DataAvailabilityHeader.hash`.
func (dah *DataAvailabilityHeader) Hash() []byte {
	if dah == nil {
		return merkle.HashFromByteSlices(nil)
	}
	if len(dah.hash) != 0 {
		return dah.hash
	}

	rowsCount := len(dah.RowsRoots)
	slices := make([][]byte, rowsCount+rowsCount)
	copy(slices[0:rowsCount], dah.RowsRoots)
	copy(slices[rowsCount:], dah.ColumnRoots)
	// The single data root is computed using a simple binary merkle tree.
	// Effectively being root(rowRoots || columnRoots):
	dah.hash = merkle.HashFromByteSlices(slices)
	return dah.hash
}

func (dah *DataAvailabilityHeader) ToProto() (*daproto.DataAvailabilityHeader, error) {
	if dah == nil {
		return nil, errors.New("nil DataAvailabilityHeader")
	}

	dahp := new(daproto.DataAvailabilityHeader)
	dahp.RowRoots = dah.RowsRoots
	dahp.ColumnRoots = dah.ColumnRoots
	return dahp, nil
}

func DataAvailabilityHeaderFromProto(dahp *daproto.DataAvailabilityHeader) (dah *DataAvailabilityHeader, err error) {
	if dahp == nil {
		return nil, errors.New("nil DataAvailabilityHeader")
	}

	dah = new(DataAvailabilityHeader)
	dah.RowsRoots = dahp.RowRoots
	dah.ColumnRoots = dahp.ColumnRoots

	return dah, dah.ValidateBasic()
}

// ValidateBasic runs stateless checks on the DataAvailabilityHeader.
func (dah *DataAvailabilityHeader) ValidateBasic() error {
	if dah == nil {
		return errors.New("nil data availability header is not valid")
	}
	if len(dah.ColumnRoots) < minExtendedSquareWidth || len(dah.RowsRoots) < minExtendedSquareWidth {
		return fmt.Errorf(
			"minimum valid DataAvailabilityHeader has at least %d row and column roots",
			minExtendedSquareWidth,
		)
	}
	if len(dah.ColumnRoots) > maxExtendedSquareWidth || len(dah.RowsRoots) > maxExtendedSquareWidth {
		return fmt.Errorf(
			"maximum valid DataAvailabilityHeader has at most %d row and column roots",
			maxExtendedSquareWidth,
		)
	}
	if len(dah.ColumnRoots) != len(dah.RowsRoots) {
		return fmt.Errorf(
			"unequal number of row and column roots: row %d col %d",
			len(dah.RowsRoots),
			len(dah.ColumnRoots),
		)
	}
	if err := types.ValidateHash(dah.Hash()); err != nil {
		return fmt.Errorf("wrong hash: %v", err)
	}

	return nil
}

func (dah *DataAvailabilityHeader) IsZero() bool {
	if dah == nil {
		return true
	}
	return len(dah.ColumnRoots) == 0 || len(dah.RowsRoots) == 0
}

// MinDataAvailabilityHeader returns the minimum valid data availability header.
// It is equal to the data availability header for an empty block
func MinDataAvailabilityHeader() DataAvailabilityHeader {
	shares := GenerateEmptyShares(appconsts.MinShareCount)
	eds, err := ExtendShares(appconsts.DefaultMinSquareSize, shares)
	if err != nil {
		panic(err)
	}
	dah := NewDataAvailabilityHeader(eds)
	return dah
}

// GenerateEmptyShares generate an array of empty shares
func GenerateEmptyShares(size int) [][]byte {
	result := make([][]byte, size)
	for i := 0; i < size; i++ {
		result[i] = shares.TailPaddingShare()
	}
	return result
}
