package internal

import (
	"context"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/syrilster/migrate-leave-krow-to-xero/internal/xero"
)

type MockXeroClient struct {
	mock.Mock
}

func TestLeaveMigration(t *testing.T) {
	var annualLeave = xero.LeaveBalance{
		LeaveType:     "Annual Leave",
		LeaveTypeID:   "73f37030-b1ed-bb37-0242ac130002",
		NumberOfUnits: 20,
		TypeOfUnits:   "Hours",
	}

	var personalLeave = xero.LeaveBalance{
		LeaveType:     "Personal/Carer's Leave",
		LeaveTypeID:   "ac62f6ec-a3cd-11ea-bb37-0242ac130002",
		NumberOfUnits: 20,
		TypeOfUnits:   "Hours",
	}

	t.Run("Success", func(t *testing.T) {
		_, b, _, _ := runtime.Caller(0)
		basePath := filepath.Dir(b)
		dir := path.Join(path.Dir(basePath), ".")
		tenantID := "111111"
		empID := "123456"
		var connectionResp = []xero.Connection{
			{
				TenantID:   tenantID,
				TenantType: "Org",
				OrgName:    "DigIO",
			},
		}

		empResp := &xero.EmpResponse{
			Status: "Active",
			Employees: []xero.Employee{
				{
					EmployeeID:        empID,
					FirstName:         "Syril",
					LastName:          "Sadasivan",
					Status:            "Active",
					PayrollCalendarID: "4567891011",
					LeaveBalance: []xero.LeaveBalance{
						annualLeave,
						personalLeave,
					},
				},
			},
		}

		payRollCalendarResp := &xero.PayrollCalendarResponse{
			PayrollCalendars: []xero.PayrollCalendar{
				{
					PayrollCalendarID: "4567891011",
					PaymentDate:       "/Date(632102400000+0000)/",
				},
			},
		}

		leaveBalResp := &xero.LeaveBalanceResponse{Employees: empResp.Employees}

		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), tenantID).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), tenantID).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), tenantID, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), tenantID, mock.Anything).Return(nil)
		xlsLocation := dir + "/test/digioLeave.xlsx"
		service := NewService(mockClient, xlsLocation)

		err := service.MigrateLeaveKrowToXero(context.Background())
		assert.Nil(t, err)
	})
}

func (m MockXeroClient) GetEmployees(ctx context.Context, tenantID string) (*xero.EmpResponse, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(*xero.EmpResponse), args.Error(1)
}

func (m MockXeroClient) GetConnections(ctx context.Context) ([]xero.Connection, error) {
	args := m.Called(ctx)
	return args.Get(0).([]xero.Connection), args.Error(1)
}

func (m MockXeroClient) EmployeeLeaveBalance(ctx context.Context, tenantID string, empID string) (*xero.LeaveBalanceResponse, error) {
	args := m.Called(ctx, tenantID, empID)
	return args.Get(0).(*xero.LeaveBalanceResponse), args.Error(1)
}

func (m MockXeroClient) EmployeeLeaveApplication(ctx context.Context, tenantID string, request xero.LeaveApplicationRequest) error {
	args := m.Called(ctx, tenantID, request)
	return args.Error(0)
}

func (m MockXeroClient) GetPayrollCalendars(ctx context.Context, tenantID string) (*xero.PayrollCalendarResponse, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(*xero.PayrollCalendarResponse), args.Error(1)
}
