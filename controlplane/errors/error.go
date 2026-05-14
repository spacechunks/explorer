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
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoiface"
)

/*
 * common errors
 */

var (
	ErrNotFound         = New(codes.NotFound, "resource does not exist")
	ErrAlreadyExists    = New(codes.AlreadyExists, "resource already exists")
	ErrInvalidPageSize  = New(codes.InvalidArgument, "page size is invalid")
	ErrInvalidPageToken = New(codes.InvalidArgument, "page token is invalid")
)

/*
 * auth related errors
 */

var (
	ErrAuthHeaderMissing = New(codes.Unauthenticated, "authorization header is missing")
	ErrInvalidToken      = New(codes.Unauthenticated, "invalid token")
	ErrPermissionDenied  = New(codes.PermissionDenied, "permission denied")
)

/*
 * chunk related errors
 */

var (
	ErrChunkNotFound              = New(codes.NotFound, "chunk does not exist")
	ErrTooManyTags                = New(codes.InvalidArgument, "too many tags")
	ErrNameTooLong                = New(codes.InvalidArgument, "name is too long")
	ErrDescriptionTooLong         = New(codes.InvalidArgument, "description is too long")
	ErrInvalidChunkID             = New(codes.InvalidArgument, "chunk id is invalid")
	ErrInvalidName                = New(codes.InvalidArgument, "name is invalid")
	ErrInvalidThumbnailFormat     = New(codes.InvalidArgument, "thumbnail image must be png")
	ErrInvalidThumbnailDimensions = New(codes.InvalidArgument, "thumbnail must be 512x512 pixels")
	ErrInvalidThumbnailSize       = New(codes.InvalidArgument, "thumbnail size too big")
)

/*
 * flavor related errors
 */

var (
	ErrFlavorNameExists             = New(codes.AlreadyExists, "flavor name already exists")
	ErrFlavorVersionExists          = New(codes.AlreadyExists, "flavor version already exists")
	ErrMinecraftVersionNotSupported = New(codes.FailedPrecondition, "minecraft version not found")
	ErrHashMismatch                 = New(codes.FailedPrecondition, "hash does not match")
	ErrInvalidHash                  = New(codes.InvalidArgument, "invalid hash")
	ErrFlavorFilesNotUploaded       = New(codes.FailedPrecondition, "flavor files have not been uploaded")
	ErrFlavorFilesUploaded          = New(codes.AlreadyExists, "flavor files have already been uploaded")
	ErrFlavorVersionNotFound        = New(codes.NotFound, "flavor version does not exist")
	ErrChangeSetTarballTooBig       = New(codes.InvalidArgument, "tarball size exceeds maximum allowed")
)

type InvalidPathViolation struct {
	Field string
	Path  string
}

func InvalidPath(violations ...InvalidPathViolation) Error {
	fieldViolations := make([]*errdetails.BadRequest_FieldViolation, 0, len(violations))
	for _, violation := range violations {
		field := violation.Field
		if field == "" {
			field = "version.file_hashes.path"
		}

		fieldViolations = append(fieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       field,
			Description: fmt.Sprintf("path %q must be a relative path within the flavor version", violation.Path),
			Reason:      "INVALID_PATH",
		})
	}

	return New(codes.InvalidArgument, "invalid path", &errdetails.BadRequest{
		FieldViolations: fieldViolations,
	})
}

/*
 * instance related errors
 */

var (
	ErrInvalidInstanceID = New(codes.InvalidArgument, "invalid instance id")
	ErrInstanceNotFound  = New(codes.NotFound, "instance not found")
	ErrNodeKeyMissing    = New(codes.InvalidArgument, "node key is missing")
	ErrNoSlotsAvailable  = New(codes.ResourceExhausted, "no slots available on any node")
)

type Error struct {
	Message string
	Detail  protoiface.MessageV1
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
	e.Code = codes.Internal
	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			e.Message = arg
		case codes.Code:
			e.Code = arg
		case protoiface.MessageV1:
			e.Detail = arg
		default:
			continue
		}
	}
	return e
}
