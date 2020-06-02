package internal

import (
	"context"
	"path"
	"path/filepath"
	"runtime"
	"strings"
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

	tenantID := "111111"
	empID := "123456"
	var connectionResp = []xero.Connection{
		{
			TenantID:   tenantID,
			TenantType: "Org",
			OrgName:    "DigIO",
		},
		{
			TenantID:   tenantID,
			TenantType: "Org",
			OrgName:    "Mantel Group",
		},
		{
			TenantID:   tenantID,
			TenantType: "Org",
			OrgName:    "CMD",
		},
		{
			TenantID:   tenantID,
			TenantType: "Org",
			OrgName:    "Eliiza",
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

	t.Run("Success", func(t *testing.T) {
		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), tenantID).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), tenantID).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), tenantID, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), tenantID, mock.Anything).Return(nil)
		xlsLocation := getProjectRoot() + "/test/digioLeave.xlsx"
		service := NewService(mockClient, xlsLocation)

		err := service.MigrateLeaveKrowToXero(context.Background())
		assert.Nil(t, err)
	})

	t.Run("Error when ORG is missing in Xero", func(t *testing.T) {
		var connectionResp = []xero.Connection{
			{
				TenantID:   tenantID,
				TenantType: "Org",
				OrgName:    "DigIO",
			},
		}

		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), tenantID).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), tenantID).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), tenantID, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), tenantID, mock.Anything).Return(nil)
		xlsLocation := getProjectRoot() + "/test/AllOrg.xlsx"
		service := NewService(mockClient, xlsLocation)

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, true, contains(errRes, "Failed to get ORG details from Xero. OrgName: Eliiza"))
		assert.Equal(t, true, contains(errRes, "Failed to get ORG details from Xero. OrgName: CMD"))
	})

	t.Run("Error when employee does not have the applied leave type configured in Xero", func(t *testing.T) {
		expectedError := "leave type Personal/Carer's Leave not found in Xero for Employee: Syril Sadasivan"

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
					},
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
		xlsLocation := getProjectRoot() + "/test/digioLeave.xlsx"
		service := NewService(mockClient, xlsLocation)

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, true, contains(errRes, expectedError))
	})

	t.Run("Error when employee is missing in Xero", func(t *testing.T) {
		expectedError := "employee not found in Xero: Stina Anderson"

		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), tenantID).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), tenantID).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), tenantID, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), tenantID, mock.Anything).Return(nil)
		xlsLocation := getProjectRoot() + "/test/AllOrg.xlsx"
		service := NewService(mockClient, xlsLocation)

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, 12, len(errRes))
		assert.Equal(t, true, contains(errRes, expectedError))
	})
}

func contains(errors []string, errStr string) bool {
	for _, s := range errors {
		if strings.Contains(s, errStr) {
			return true
		}
	}
	return false
}

func getProjectRoot() string {
	_, b, _, _ := runtime.Caller(0)
	basePath := filepath.Dir(b)
	dir := path.Join(path.Dir(basePath), ".")
	return dir
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
