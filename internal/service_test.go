package internal

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
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

	var compassionateLeave = xero.LeaveBalance{
		LeaveType:     "Compassionate Leave (paid)",
		LeaveTypeID:   "df62f6ec-a3cd-11ea-bb37-0242ac1300123",
		NumberOfUnits: 8,
		TypeOfUnits:   "Hours",
	}

	var juryDurtyLeave = xero.LeaveBalance{
		LeaveType:     "Jury Duty",
		LeaveTypeID:   "ca62f6ec-a3cd-11ea-bb37-0242ac130005",
		NumberOfUnits: 8,
		TypeOfUnits:   "Hours",
	}

	digIOTenantID := "111111"
	eliizaTenantID := "222222"
	cmdTenantID := "333333"
	mantelTenantID := "4444444"
	empID := "123456"
	var connectionResp = []xero.Connection{
		{
			TenantID:   digIOTenantID,
			TenantType: "Org",
			OrgName:    "DigIO",
		},
		{
			TenantID:   mantelTenantID,
			TenantType: "Org",
			OrgName:    "Mantel Group",
		},
		{
			TenantID:   cmdTenantID,
			TenantType: "Org",
			OrgName:    "CMD",
		},
		{
			TenantID:   eliizaTenantID,
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
	mockClient := new(MockXeroClient)
	sesClient := ses.New(session.New())
	mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
	mockClient.On("GetEmployees", context.Background(), digIOTenantID).Return(empResp, nil)
	mockClient.On("GetPayrollCalendars", context.Background(), digIOTenantID).Return(payRollCalendarResp, nil)
	mockClient.On("EmployeeLeaveBalance", context.Background(), digIOTenantID, empID).Return(leaveBalResp, nil)
	mockClient.On("EmployeeLeaveApplication", context.Background(), digIOTenantID, mock.Anything).Return(nil)

	t.Run("Success", func(t *testing.T) {
		xlsLocation := getProjectRoot() + "/test/digio_leave.xlsx"
		service := NewService(mockClient, xlsLocation, sesClient, "")

		err := service.MigrateLeaveKrowToXero(context.Background())
		assert.Nil(t, err)
	})

	t.Run("Error when employee has insufficient leave balance for leave type Jury Duty and compassionate leave", func(t *testing.T) {
		expectedResp := "Employee: Syril Sadasivan has insufficient Leave balance for Leave type Compassionate Leave (paid) requested for 8 hours"
		xlsLocation := getProjectRoot() + "/test/digio_various_leave.xlsx"

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
						compassionateLeave,
						juryDurtyLeave,
					},
				},
			},
		}

		leaveBalResp := &xero.LeaveBalanceResponse{Employees: empResp.Employees}
		mockClient := new(MockXeroClient)
		sesClient := ses.New(session.New())
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), digIOTenantID).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), digIOTenantID).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), digIOTenantID, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), digIOTenantID, mock.Anything).Return(nil)

		service := NewService(mockClient, xlsLocation, sesClient, "")
		err := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, err)
		assert.Equal(t, 2, len(err))
		assert.Equal(t, true, contains(err, expectedResp))
	})

	t.Run("Error when ORG is missing in Xero", func(t *testing.T) {
		cResp := []xero.Connection{
			{
				TenantID:   digIOTenantID,
				TenantType: "Org",
				OrgName:    "DigIO",
			},
		}
		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(cResp, nil)
		mockClient.On("GetEmployees", context.Background(), digIOTenantID).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), digIOTenantID).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), digIOTenantID, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), digIOTenantID, mock.Anything).Return(nil)
		xlsLocation := getProjectRoot() + "/test/all_org.xlsx"
		service := NewService(mockClient, xlsLocation, sesClient, "")

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, true, contains(errRes, "Failed to get ORG details from Xero. OrgName: Eliiza"))
		assert.Equal(t, true, contains(errRes, "Failed to get ORG details from Xero. OrgName: CMD"))
	})

	t.Run("Error when employee does not have the applied leave type configured in Xero", func(t *testing.T) {
		expectedError := "Leave type Personal/Carer's Leave not found/configured in Xero for Employee Name: Syril Sadasivan "

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
		mockClient.On("GetEmployees", context.Background(), digIOTenantID).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), digIOTenantID).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), digIOTenantID, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), digIOTenantID, mock.Anything).Return(nil)
		xlsLocation := getProjectRoot() + "/test/digio_leave.xlsx"
		service := NewService(mockClient, xlsLocation, sesClient, "")

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, true, contains(errRes, expectedError))
	})

	t.Run("Error when employee is missing in Xero", func(t *testing.T) {
		expectedError := "Employee not found in Xero. Employee Name: Stina Anderson "

		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), mock.Anything).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), mock.Anything).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), mock.Anything, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), mock.Anything, mock.Anything).Return(nil)

		xlsLocation := getProjectRoot() + "/test/all_org.xlsx"
		service := NewService(mockClient, xlsLocation, sesClient, "")

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, 12, len(errRes))
		assert.Equal(t, true, contains(errRes, expectedError))
	})

	t.Run("Error when payroll calendar settings not found for employee", func(t *testing.T) {
		expectedError := "Failed to fetch employee payroll calendar settings from Xero. Employee Name: Stina Anderson "

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
				{
					EmployeeID:        "45678974111",
					FirstName:         "Stina",
					LastName:          "Anderson",
					Status:            "Active",
					PayrollCalendarID: "789845651232",
					LeaveBalance: []xero.LeaveBalance{
						annualLeave,
						personalLeave,
					},
				},
			},
		}

		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), mock.Anything).Return(empResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), mock.Anything).Return(payRollCalendarResp, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), mock.Anything, empID).Return(leaveBalResp, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), mock.Anything, mock.Anything).Return(nil)

		xlsLocation := getProjectRoot() + "/test/cmd_leave.xlsx"
		service := NewService(mockClient, xlsLocation, sesClient, "")

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, 1, len(errRes))
		assert.Equal(t, true, contains(errRes, expectedError))
	})

	t.Run("Error when failed to post leave request to xero", func(t *testing.T) {
		expectedError := "Failed to post Leave application to xero for employee Stina Anderson. Check if entered leave type/unpaid leave is configured."

		digIOEmpResp := &xero.EmpResponse{
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
		cmdEmpResp := &xero.EmpResponse{
			Status: "Active",
			Employees: []xero.Employee{
				{
					EmployeeID:        "45678974111",
					FirstName:         "Stina",
					LastName:          "Anderson",
					Status:            "Active",
					PayrollCalendarID: "789845651232",
					LeaveBalance: []xero.LeaveBalance{
						annualLeave,
						personalLeave,
					},
				},
			},
		}

		digIOLeaveBal := &xero.LeaveBalanceResponse{Employees: digIOEmpResp.Employees}
		cmdLeaveBal := &xero.LeaveBalanceResponse{Employees: cmdEmpResp.Employees}
		digIOPayrollCal := &xero.PayrollCalendarResponse{
			PayrollCalendars: []xero.PayrollCalendar{
				{
					PayrollCalendarID: "4567891011",
					PaymentDate:       "/Date(632102400000+0000)/",
				},
			},
		}

		cmdPayrollCal := &xero.PayrollCalendarResponse{
			PayrollCalendars: []xero.PayrollCalendar{
				{
					PayrollCalendarID: "789845651232",
					PaymentDate:       "/Date(632102400000+0000)/",
				},
			},
		}

		mockClient := new(MockXeroClient)
		mockClient.On("GetConnections", context.Background()).Return(connectionResp, nil)
		mockClient.On("GetEmployees", context.Background(), digIOTenantID).Return(digIOEmpResp, nil)
		mockClient.On("GetEmployees", context.Background(), cmdTenantID).Return(cmdEmpResp, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), digIOTenantID).Return(digIOPayrollCal, nil)
		mockClient.On("GetPayrollCalendars", context.Background(), cmdTenantID).Return(cmdPayrollCal, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), digIOTenantID, empID).Return(digIOLeaveBal, nil)
		mockClient.On("EmployeeLeaveBalance", context.Background(), cmdTenantID, "45678974111").Return(cmdLeaveBal, nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), digIOTenantID, mock.Anything).Return(nil)
		mockClient.On("EmployeeLeaveApplication", context.Background(), cmdTenantID, mock.Anything).Return(errors.New("something went wrong"))

		xlsLocation := getProjectRoot() + "/test/cmd_leave.xlsx"
		service := NewService(mockClient, xlsLocation, sesClient, "")

		errRes := service.MigrateLeaveKrowToXero(context.Background())
		assert.NotNil(t, errRes)
		assert.Equal(t, 1, len(errRes))
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
