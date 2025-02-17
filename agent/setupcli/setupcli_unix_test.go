// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	rMock "github.com/aws/amazon-ssm-agent/agent/managedInstances/registration/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/configurationmanager"
	cmMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/configurationmanager/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/downloadmanager"
	dmMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/downloadmanager/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/helpers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers"
	pmMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/registermanager"
	rmMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/registermanager/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	smMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
	vmMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers/mocks"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const breakOutWithPanicMessage = "BREAKOUT_WITH_PANIC"
const testArtifactsPath = "SomeArtifacts"

func storeMockedFunctions() func() {
	getPackageManagerStorage := getPackageManager
	getConfigurationManagerStorage := getConfigurationManager
	getServiceManagerStorage := getServiceManager
	getRegisterManagerStorage := getRegisterManager
	getRegistrationInfoStorage := getRegistrationInfo
	hasElevatedPermissions = func() error {
		return nil
	}
	return func() {
		getPackageManager = getPackageManagerStorage
		getConfigurationManager = getConfigurationManagerStorage
		getServiceManager = getServiceManagerStorage
		getRegisterManager = getRegisterManagerStorage
		getRegistrationInfo = getRegistrationInfoStorage
	}
}

func setArgsAndRestore(args ...string) func() {
	var oldArgs = make([]string, len(os.Args))
	copy(oldArgs, os.Args)

	os.Args = args
	return func() {
		os.Args = oldArgs
	}
}

func initializeArgs() {
	os.Setenv("SSM_ARTIFACTS_PATH", testArtifactsPath)
	os.Setenv("AWS_REGION", "SomeRegion")
	os.Setenv("SSM_REGISTRATION_ROLE", "SomeRole")
	os.Setenv("SSM_RESOURCE_TAGS", "SomeTags")
	os.Setenv("SSM_OVERRIDE_EXISTING_REGISTRATION", "false")
	os.Setenv("", "")

	hasElevatedPermissions = func() error {
		return nil
	}

	svcMgrStopAgent = servicemanagers.StopAgent
	helperInstallAgent = helpers.InstallAgent
	helperUnInstallAgent = helpers.UninstallAgent
	startAgent = servicemanagers.StartAgent
	svcMgrStopAgent = servicemanagers.StopAgent
}

func TestMain_ErrorGetServiceManager(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-shutdown", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		return &pmMock.IPackageManager{}, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		return nil, fmt.Errorf("SomeError")
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to determine service manager")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_FailedShutdown(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()
	defer setArgsAndRestore("/some/path/setupcli", "-shutdown", "-env", "greengrass")()

	svcMgrStopAgent = servicemanagers.StopAgent

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		return &pmMock.IPackageManager{}, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(fmt.Errorf("SomeError"))
		return managerMock, nil
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		managerMock := &vmMock.IVerificationManager{}
		managerMock.On("VerifyAgentSignature", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to shut down agent")
		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_SuccessShutdown(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-shutdown", "-env", "greengrass")()

	svcMgrStopAgent = servicemanagers.StopAgent

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		return &pmMock.IPackageManager{}, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Stopped, nil)
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		managerMock := &vmMock.IVerificationManager{}
		managerMock.On("VerifyAgentSignature", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Fail(t, "Should not get panic")
		}
	}()

	main()
	assert.True(t, true, "Should never reach here because of exit")
}

func TestMain_Install_FailedCheckAgentInstalled(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-install", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(false, fmt.Errorf("SomeError"))
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		return managerMock, nil
	}

	getConfigurationManager = func() configurationmanager.IConfigurationManager {
		managerMock := &cmMock.IConfigurationManager{}
		managerMock.On("IsConfigAvailable", "").Return(true, nil)
		managerMock.On("CreateUpdateAgentConfigWithOnPremIdentity").Return(nil)

		return managerMock
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		managerMock := &vmMock.IVerificationManager{}
		managerMock.On("VerifyAgentSignature", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getDownloadManager = func(log log.T, region string, manifestUrl string, updateInfo updateinfo.T, setupCLIArtifactsPath string, isNano bool) downloadmanager.IDownloadManager {
		managerMock := &dmMock.IDownloadManager{}
		managerMock.On("DownloadArtifacts", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to determine if agent is installed")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Install_AgentConfigured_ConfigurationNotLoadable_Fail(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-install", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(false, fmt.Errorf("SomeError"))
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		return managerMock, nil
	}

	getConfigurationManager = func() configurationmanager.IConfigurationManager {
		managerMock := &cmMock.IConfigurationManager{}
		managerMock.On("IsConfigAvailable", "").Return(false, fmt.Errorf("failed to configure agent"))
		managerMock.On("CreateUpdateAgentConfigWithOnPremIdentity").Return(nil)
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "failed to configure agent")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Install_AgentNotConfigured_DefaultConfigurationMissing_Fail(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-install", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(false, fmt.Errorf("SomeError"))
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		return managerMock, nil
	}

	getConfigurationManager = func() configurationmanager.IConfigurationManager {
		managerMock := &cmMock.IConfigurationManager{}
		managerMock.On("IsConfigAvailable", "").Return(false, nil)
		managerMock.On("IsConfigAvailable", testArtifactsPath).Return(false, fmt.Errorf("failed to configure agent"))
		managerMock.On("CreateUpdateAgentConfigWithOnPremIdentity").Return(nil)
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "failed to configure agent")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Install_AgentIsInstalled_UninstallAgentFailed(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-install", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("UninstallAgent", mock.Anything, "").Return(fmt.Errorf("SomeUninstallError"))
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getConfigurationManager = func() configurationmanager.IConfigurationManager {
		managerMock := &cmMock.IConfigurationManager{}
		managerMock.On("IsConfigAvailable", "").Return(true, nil)
		managerMock.On("CreateUpdateAgentConfigWithOnPremIdentity").Return(nil)
		return managerMock
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		managerMock := &vmMock.IVerificationManager{}
		managerMock.On("VerifyAgentSignature", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getDownloadManager = func(log log.T, region string, manifestUrl string, updateInfo updateinfo.T, setupCLIArtifactsPath string, isNano bool) downloadmanager.IDownloadManager {
		managerMock := &dmMock.IDownloadManager{}
		managerMock.On("DownloadArtifacts", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to uninstall the agent")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Install_AgentIsInstalled_UninstallSuccess_InstallFailed(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-install", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("UninstallAgent", mock.Anything, "").Return(nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)
		managerMock.On("GetFilesReqForInstall", mock.Anything).Return([]string{})
		managerMock.On("InstallAgent", mock.Anything, mock.Anything).Return(fmt.Errorf("FailedInstallAgent"))
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getConfigurationManager = func() configurationmanager.IConfigurationManager {
		managerMock := &cmMock.IConfigurationManager{}
		managerMock.On("IsConfigAvailable", "").Return(true, nil)
		managerMock.On("CreateUpdateAgentConfigWithOnPremIdentity").Return(nil)
		return managerMock
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		managerMock := &vmMock.IVerificationManager{}
		managerMock.On("VerifyAgentSignature", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getDownloadManager = func(log log.T, region string, manifestUrl string, updateInfo updateinfo.T, setupCLIArtifactsPath string, isNano bool) downloadmanager.IDownloadManager {
		managerMock := &dmMock.IDownloadManager{}
		managerMock.On("DownloadArtifacts", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to install agent")
		assert.Equal(t, "FailedInstallAgent", args[0].(error).Error())

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Install_AgentIsInstalled_UninstallSuccess_InstallSuccess_ReloadServiceFailed(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-install", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("UninstallAgent", mock.Anything, "").Return(nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)
		managerMock.On("GetFilesReqForInstall", mock.Anything).Return([]string{})
		managerMock.On("InstallAgent", mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(nil)
		managerMock.On("ReloadManager").Return(fmt.Errorf("FailedReloadManager"))
		return managerMock, nil
	}

	getConfigurationManager = func() configurationmanager.IConfigurationManager {
		managerMock := &cmMock.IConfigurationManager{}
		managerMock.On("IsConfigAvailable", "").Return(true, nil)
		managerMock.On("CreateUpdateAgentConfigWithOnPremIdentity").Return(nil)
		return managerMock
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		managerMock := &vmMock.IVerificationManager{}
		managerMock.On("VerifyAgentSignature", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getDownloadManager = func(log log.T, region string, manifestUrl string, updateInfo updateinfo.T, setupCLIArtifactsPath string, isNano bool) downloadmanager.IDownloadManager {
		managerMock := &dmMock.IDownloadManager{}
		managerMock.On("DownloadArtifacts", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to install agent")
		assert.Contains(t, args[0].(error).Error(), "FailedReloadManager")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Install_AgentNotInstalled_InstallSuccess(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-install", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(false, nil)
		managerMock.On("GetFilesReqForInstall", mock.Anything).Return([]string{})
		managerMock.On("InstallAgent", mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("ReloadManager").Return(nil)
		managerMock.On("GetAgentStatus").Return(common.Running, nil)
		managerMock.On("StartAgent").Return(nil)

		return managerMock, nil
	}

	getConfigurationManager = func() configurationmanager.IConfigurationManager {
		managerMock := &cmMock.IConfigurationManager{}
		managerMock.On("IsConfigAvailable", "").Return(true, nil)
		managerMock.On("CreateUpdateAgentConfigWithOnPremIdentity").Return(nil)
		return managerMock
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		managerMock := &vmMock.IVerificationManager{}
		managerMock.On("VerifyAgentSignature", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	getDownloadManager = func(log log.T, region string, manifestUrl string, updateInfo updateinfo.T, setupCLIArtifactsPath string, isNano bool) downloadmanager.IDownloadManager {
		managerMock := &dmMock.IDownloadManager{}
		managerMock.On("DownloadArtifacts", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Fail(t, "Should not get panic")
		}
	}()

	main()
	assert.True(t, true, "Should never reach here because of exit")
}

func TestMain_Register_ErrorCheckingAgentInstalled(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(false, fmt.Errorf("SomeError"))
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)

		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(nil)
		return managerMock, nil
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to determine if agent is installed")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(false, nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		return managerMock, nil
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Agent must be installed before attempting to register")
		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled_OverrideNotSet_StartAgentFailed(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(nil)
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Stopped, nil)
		managerMock.On("StartAgent").Return(fmt.Errorf("SomeError"))
		return managerMock, nil
	}

	getRegistrationInfo = func() registration.IOnpremRegistrationInfo {
		registrationMock := &rMock.IOnpremRegistrationInfo{}
		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("SomeInstanceId")
		return registrationMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to start agent")
		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()
	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled_AgentRegistered_OverrideNotSet_StartAgentSuccess(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)

		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("StopAgent", mock.Anything, mock.Anything).Return(nil)
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Running, nil)
		managerMock.On("StartAgent").Return(nil)
		return managerMock, nil
	}

	getRegistrationInfo = func() registration.IOnpremRegistrationInfo {
		registrationMock := &rMock.IOnpremRegistrationInfo{}

		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("SomeInstanceId")
		return registrationMock
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			fmt.Println(errInterface)
			assert.Fail(t, "Should not get panic")
		}
	}()

	main()
	assert.True(t, true, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled_AgentNotRegistered_OverrideNotSet_FailedStopAgent(t *testing.T) {
	initializeArgs()
	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Running, nil)
		managerMock.On("StopAgent").Return(fmt.Errorf("SomeError"))
		return managerMock, nil
	}

	getRegistrationInfo = func() registration.IOnpremRegistrationInfo {
		registrationMock := &rMock.IOnpremRegistrationInfo{}
		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("")
		return registrationMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to stop agent")
		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()

	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled_AgentRegistered_OverrideSet_FailedRegisterAgent(t *testing.T) {
	initializeArgs()
	os.Setenv("SSM_OVERRIDE_EXISTING_REGISTRATION", "true")

	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)

		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}

		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Stopped, nil)
		managerMock.On("StopAgent").Return(nil)
		return managerMock, nil
	}

	getRegistrationInfo = func() registration.IOnpremRegistrationInfo {
		registrationMock := &rMock.IOnpremRegistrationInfo{}

		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("SomeInstanceId")
		return registrationMock
	}

	getRegisterManager = func() registermanager.IRegisterManager {
		managerMock := &rmMock.IRegisterManager{}

		managerMock.On("RegisterAgent", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("SomeError"))
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to register agent")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()

	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled_AgentRegistered_OverrideSet_FailedToStartAgent(t *testing.T) {
	initializeArgs()
	os.Setenv("SSM_OVERRIDE_EXISTING_REGISTRATION", "true")

	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)

		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Stopped, nil).Times(1)
		managerMock.On("StopAgent").Return(nil)
		managerMock.On("StartAgent").Return(fmt.Errorf("SomeError"))
		return managerMock, nil
	}

	getRegistrationInfo = func() registration.IOnpremRegistrationInfo {
		registrationMock := &rMock.IOnpremRegistrationInfo{}
		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("SomeInstanceId")
		registrationMock.On("ReloadInstanceInfo", mock.Anything, "", mock.Anything).Return()
		return registrationMock
	}

	getRegisterManager = func() registermanager.IRegisterManager {
		managerMock := &rmMock.IRegisterManager{}

		managerMock.On("RegisterAgent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}

	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to start agent")

		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()

	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled_AgentRegistered_OverrideSet_FailedToGetNewInstanceId(t *testing.T) {
	initializeArgs()
	os.Setenv("SSM_OVERRIDE_EXISTING_REGISTRATION", "true")

	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("IsAgentInstalled").Return(true, nil)
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)

		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Stopped, nil).Times(1)
		managerMock.On("StopAgent").Return(nil)
		managerMock.On("StartAgent").Return(nil)
		managerMock.On("GetAgentStatus").Return(common.Running, nil).Times(1)
		return managerMock, nil
	}

	getRegistrationInfo = func() registration.IOnpremRegistrationInfo {
		registrationMock := &rMock.IOnpremRegistrationInfo{}
		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("SomeInstanceId").Once()
		registrationMock.On("ReloadInstanceInfo", mock.Anything, "", mock.Anything).Return()
		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("").Once()
		return registrationMock
	}

	getRegisterManager = func() registermanager.IRegisterManager {
		managerMock := &rmMock.IRegisterManager{}
		managerMock.On("RegisterAgent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}
	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		verificationManager := &vmMock.IVerificationManager{}
		verificationManager.On("VerifyAgentSignature", mock.Anything, mock.Anything).Return(nil)
		return verificationManager, nil
	}
	osExit = func(exitCode int, log log.T, message string, args ...interface{}) {
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, message, "Failed to get new instance id")
		panic(breakOutWithPanicMessage)
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Equal(t, breakOutWithPanicMessage, errInterface)
		}
	}()

	main()
	assert.True(t, false, "Should never reach here because of exit")
}

func TestMain_Register_AgentNotInstalled_AgentRegistered_OverrideSet_Success(t *testing.T) {
	initializeArgs()
	os.Setenv("SSM_OVERRIDE_EXISTING_REGISTRATION", "true")

	defer storeMockedFunctions()()

	defer setArgsAndRestore("/some/path/setupcli", "-register", "-env", "greengrass")()

	getPackageManager = func(log.T) (packagemanagers.IPackageManager, error) {
		managerMock := &pmMock.IPackageManager{}
		managerMock.On("GetInstalledAgentVersion").Return("2.1.2.2", nil)
		managerMock.On("IsAgentInstalled").Return(true, nil)
		return managerMock, nil
	}

	getServiceManager = func(log.T) (servicemanagers.IServiceManager, error) {
		managerMock := &smMock.IServiceManager{}
		managerMock.On("GetName").Return("ServiceManagerName")
		managerMock.On("GetAgentStatus").Return(common.Stopped, nil).Times(1)
		managerMock.On("StopAgent").Return(nil)
		managerMock.On("StartAgent").Return(nil)
		managerMock.On("GetAgentStatus").Return(common.Running, nil).Times(1)
		return managerMock, nil
	}

	getRegistrationInfo = func() registration.IOnpremRegistrationInfo {
		registrationMock := &rMock.IOnpremRegistrationInfo{}
		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("SomeInstanceId").Once()
		registrationMock.On("ReloadInstanceInfo", mock.Anything, "", mock.Anything).Return()
		registrationMock.On("InstanceID", mock.Anything, "", mock.Anything).Return("NewInstanceId").Once()
		return registrationMock
	}

	getRegisterManager = func() registermanager.IRegisterManager {
		managerMock := &rmMock.IRegisterManager{}
		managerMock.On("RegisterAgent", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		return managerMock
	}

	getVerificationManager = func() (verificationmanagers.IVerificationManager, error) {
		verificationManager := &vmMock.IVerificationManager{}
		verificationManager.On("VerifyAgentSignature", mock.Anything, mock.Anything).Return(nil)
		return verificationManager, nil
	}

	defer func() {
		if errInterface := recover(); errInterface != nil {
			assert.Fail(t, "Should not get panic")
		}
	}()

	main()
	assert.True(t, true, "Should never reach here because of exit")
}
