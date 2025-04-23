/*
 Explorer Platform, a platform for hosting and discovering Minecraft servers.
 Copyright (C) 2024 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package errors

import (
	"github.com/gogo/protobuf/proto"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
 * chunk related errors
 */

var (
	ErrChunkNotFound      = New(codes.NotFound, "chunk does not exist")
	ErrTooManyTags        = New(codes.InvalidArgument, "too many tags")
	ErrNameTooLong        = New(codes.InvalidArgument, "name is too long")
	ErrDescriptionTooLong = New(codes.InvalidArgument, "description is too long")
	ErrInvalidChunkID     = New(codes.InvalidArgument, "chunk id is invalid")
	ErrInvalidName        = New(codes.InvalidArgument, "name is invalid")
)

/*
 * flavor related errors
 */

var (
	ErrFlavorNameExists    = New(codes.AlreadyExists, "flavor name already exists")
	ErrFlavorVersionExists = New(codes.AlreadyExists, "flavor version already exists")
	ErrHashMismatch        = New(codes.FailedPrecondition, "hash does not match")
	ErrFilesAlreadyExist   = New(codes.AlreadyExists, "files already exist")
	ErrFlavorNotFound      = New(codes.NotFound, "flavor does not exist")
)

func FlavorVersionDuplicate(version string) Error {
	return New(
		codes.FailedPrecondition,
		"flavor version is a duplicate",
		&errdetails.ErrorInfo{
			Reason: "DUPLICATE",
			Domain: "explorer.chunks.space",
			Metadata: map[string]string{
				"version": version,
			},
		},
	)
}

/*
 * instance related errors
 */

var (
	ErrInvalidInstanceID = New(codes.InvalidArgument, "invalid instance id")
	ErrInstanceNotFound  = New(codes.NotFound, "instance not found")
	ErrNodeKeyMissing    = New(codes.InvalidArgument, "node key is missing")
)

type Error struct {
	Message string
	Detail  proto.Message
	Code    codes.Code
}

func (e Error) GRPCStatus() *status.Status {
	st := status.New(e.Code, e.Message)
	if e.Detail != nil {
		witDetails, err := st.WithDetails(e.Detail)
		if err != nil {
			return st
		}
		return witDetails
	}
	return st
}

func (e Error) Error() string {
	return e.Message
}

func New(args ...any) Error {
	e := Error{}
	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			e.Message = arg
		case codes.Code:
			e.Code = arg
		case proto.Message:
			e.Detail = arg
		default:
			continue
		}
	}
	return e
}
