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

package authz

import (
	"context"
	"fmt"

	apierrs "github.com/spacechunks/explorer/controlplane/errors"
	"github.com/spacechunks/explorer/controlplane/resource"
)

type AccessRuleOption func(*accessRules)

type accessRules struct {
	OwnershipRule *OwnershipRule
}

func WithOwnershipRule(actorID string, resource ResourceDef) AccessRuleOption {
	return func(rules *accessRules) {
		rules.OwnershipRule = &OwnershipRule{
			ActorID:  actorID,
			Resource: resource,
		}
	}
}

type OwnershipRule struct {
	Resource ResourceDef
	ActorID  string
}

type AccessEvaluator interface {
	AccessAuthorized(ctx context.Context, opts ...AccessRuleOption) error
}

type RuleEvaluator struct {
	repo Repository
}

func NewRuleEvaluator(repo Repository) *RuleEvaluator {
	return &RuleEvaluator{
		repo: repo,
	}
}

func (e RuleEvaluator) AccessAuthorized(ctx context.Context, opts ...AccessRuleOption) error {
	rules := accessRules{}
	for _, o := range opts {
		o(&rules)
	}

	if rules.OwnershipRule != nil {
		if err := e.evalOwnership(ctx, rules.OwnershipRule); err != nil {
			return fmt.Errorf("eval ownership: %w", err)
		}
	}

	// add more rule evaluations below

	return nil
}

func (e RuleEvaluator) evalOwnership(ctx context.Context, rule *OwnershipRule) error {
	var owner resource.User
	switch rule.Resource.Type {
	case ResourceTypeChunk:
		o, err := e.repo.ChunkOwner(ctx, rule.Resource.ID)
		if err != nil {
			return fmt.Errorf("chunk: %w", err)
		}
		owner = o
	case ResourceTypeFlavor:
		o, err := e.repo.FlavorOwner(ctx, rule.Resource.ID)
		if err != nil {
			return fmt.Errorf("flavor: %w", err)
		}
		owner = o
	case ResourceTypeFlavorVersion:
		o, err := e.repo.FlavorVersionOwner(ctx, rule.Resource.ID)
		if err != nil {
			return fmt.Errorf("flavor version: %w", err)
		}
		owner = o
	default:
		return fmt.Errorf("unknown resource type") // should not happen
	}

	if rule.ActorID != owner.ID {
		return apierrs.ErrPermissionDenied
	}

	return nil
}
