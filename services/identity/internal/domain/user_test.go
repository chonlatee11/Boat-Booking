package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

func TestStaffUserInputValidate(t *testing.T) {
	pierA, pierB := uuid.New(), uuid.New()

	valid := func() domain.StaffUserInput {
		return domain.StaffUserInput{
			Email:      " Staff@Example.com ",
			Name:       "Staff One",
			Role:       auth.RoleStaff,
			OperatorID: uuid.New(),
			PierIDs:    []uuid.UUID{pierA, pierB},
		}
	}

	cases := []struct {
		name    string
		mutate  func(in *domain.StaffUserInput)
		wantErr bool
	}{
		{name: "valid staff", mutate: func(in *domain.StaffUserInput) {}, wantErr: false},
		{name: "valid pier_admin", mutate: func(in *domain.StaffUserInput) { in.Role = auth.RolePierAdmin }, wantErr: false},
		{name: "role customer rejected", mutate: func(in *domain.StaffUserInput) { in.Role = auth.RoleCustomer }, wantErr: true},
		{name: "role super_admin rejected", mutate: func(in *domain.StaffUserInput) { in.Role = auth.RoleSuperAdmin }, wantErr: true},
		{name: "empty name rejected", mutate: func(in *domain.StaffUserInput) { in.Name = "   " }, wantErr: true},
		{name: "name too long rejected", mutate: func(in *domain.StaffUserInput) { in.Name = strings.Repeat("a", 101) }, wantErr: true},
		{name: "name at max length ok", mutate: func(in *domain.StaffUserInput) { in.Name = strings.Repeat("a", 100) }, wantErr: false},
		{name: "invalid email rejected", mutate: func(in *domain.StaffUserInput) { in.Email = "not-an-email" }, wantErr: true},
		{name: "empty pier_ids rejected", mutate: func(in *domain.StaffUserInput) { in.PierIDs = nil }, wantErr: true},
		{name: "too many pier_ids rejected", mutate: func(in *domain.StaffUserInput) {
			ids := make([]uuid.UUID, 51)
			for i := range ids {
				ids[i] = uuid.New()
			}
			in.PierIDs = ids
		}, wantErr: true},
		{name: "50 pier_ids ok", mutate: func(in *domain.StaffUserInput) {
			ids := make([]uuid.UUID, 50)
			for i := range ids {
				ids[i] = uuid.New()
			}
			in.PierIDs = ids
		}, wantErr: false},
		{name: "duplicate pier_ids rejected", mutate: func(in *domain.StaffUserInput) { in.PierIDs = []uuid.UUID{pierA, pierA} }, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := valid()
			c.mutate(&in)
			err := in.Validate()
			if c.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if c.wantErr && !errors.Is(err, domain.ErrInvalidArgument) {
				t.Fatalf("Validate() error = %v, want wrapping ErrInvalidArgument", err)
			}
		})
	}
}

func TestStaffUserInputValidateNormalizesEmail(t *testing.T) {
	in := domain.StaffUserInput{
		Email:      " Staff@Example.com ",
		Name:       "Staff One",
		Role:       auth.RoleStaff,
		OperatorID: uuid.New(),
		PierIDs:    []uuid.UUID{uuid.New()},
	}
	if err := in.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	if in.Email != "staff@example.com" {
		t.Errorf("Email = %q, want normalised %q", in.Email, "staff@example.com")
	}
}
