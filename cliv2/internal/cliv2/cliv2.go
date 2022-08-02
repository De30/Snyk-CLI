/*
Entry point class for the CLIv2 version.
*/
package cliv2

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"bytes"
	"github.com/snyk/cli-extension-lib-go"
	"github.com/snyk/cli-extension-lib-go/extension"
	"github.com/snyk/cli/cliv2/internal/embedded"
	"github.com/snyk/cli/cliv2/internal/embedded/cliv1"
	"github.com/snyk/cli/cliv2/internal/exit_codes"
	"github.com/snyk/cli/cliv2/internal/utils"
	"github.com/spf13/cobra"
)

type Handler int

type CLI struct {
	DebugLogger      *log.Logger
	CacheDirectory   string
	v1BinaryLocation string
	v1Version        string
	v2Version        string
	Extensions       []*extension.Extension
	ArgParserRootCmd *cobra.Command
	debugMode        bool
}

type EnvironmentWarning struct {
	message string
}

const SNYK_EXIT_CODE_OK = 0
const SNYK_EXIT_CODE_ERROR = 2
const SNYK_INTEGRATION_NAME = "CLI_V1_PLUGIN"
const SNYK_INTEGRATION_NAME_ENV = "SNYK_INTEGRATION_NAME"
const SNYK_INTEGRATION_VERSION_ENV = "SNYK_INTEGRATION_VERSION"
const SNYK_HTTPS_PROXY_ENV = "HTTPS_PROXY"
const SNYK_HTTP_PROXY_ENV = "HTTP_PROXY"
const SNYK_HTTP_NO_PROXY_ENV = "NO_PROXY"
const SNYK_NPM_PROXY_ENV = "NPM_CONFIG_PROXY"
const SNYK_NPM_HTTPS_PROXY_ENV = "NPM_CONFIG_HTTPS_PROXY"
const SNYK_NPM_HTTP_PROXY_ENV = "NPM_CONFIG_HTTP_PROXY"
const SNYK_NPM_NO_PROXY_ENV = "NPM_CONFIG_NO_PROXY"
const SNYK_NPM_ALL_PROXY = "ALL_PROXY"
const SNYK_CA_CERTIFICATE_LOCATION_ENV = "NODE_EXTRA_CA_CERTS"

const (
	V1_DEFAULT Handler = iota
	V2_VERSION Handler = iota
)

//go:embed cliv2.version
var SNYK_CLIV2_VERSION_PART string

func NewCLIv2(cacheDirectory string, extensions []*extension.Extension, argParserRootCmd *cobra.Command, debugMode bool, debugLogger *log.Logger) *CLI {
	v1BinaryLocation, err := cliv1.GetFullCLIV1TargetPath(cacheDirectory)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	cli := CLI{
		DebugLogger:      debugLogger,
		CacheDirectory:   cacheDirectory,
		v1Version:        cliv1.CLIV1Version(),
		v2Version:        strings.TrimSpace(SNYK_CLIV2_VERSION_PART),
		v1BinaryLocation: v1BinaryLocation,
		Extensions:       extensions,
		ArgParserRootCmd: argParserRootCmd,
		debugMode:        debugMode,
	}

	err = cli.ExtractV1Binary()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &cli
}

func (c *CLI) ExtractV1Binary() error {
	cliV1ExpectedSHA256 := cliv1.ExpectedSHA256()

	isValid, err := embedded.ValidateFile(c.v1BinaryLocation, cliV1ExpectedSHA256, c.DebugLogger)
	if err != nil || !isValid {
		c.DebugLogger.Println("cliv1 is not valid, start extracting ", c.v1BinaryLocation)

		err = cliv1.ExtractTo(c.v1BinaryLocation)
		if err != nil {
			return err
		}

		isValid, err := embedded.ValidateFile(c.v1BinaryLocation, cliV1ExpectedSHA256, c.DebugLogger)
		if err != nil {
			return err
		}

		if isValid {
			c.DebugLogger.Println("cliv1 is valid after extracting", c.v1BinaryLocation)
		} else {
			fmt.Println("cliv1 is not valid after sha256 check")
			return err
		}
	} else {
		c.DebugLogger.Println("cliv1 already exists and is valid at", c.v1BinaryLocation)
	}

	return nil
}

func (c *CLI) GetFullVersion() string {
	return c.v2Version + "." + c.v1Version
}

func (c *CLI) GetIntegrationName() string {
	return SNYK_INTEGRATION_NAME
}

func (c *CLI) GetBinaryLocation() string {
	return c.v1BinaryLocation
}

func (c *CLI) printVersion() {
	fmt.Println(c.GetFullVersion())
}

func (c *CLI) commandVersion(passthroughArgs []string) int {
	if utils.Contains(passthroughArgs, "--json-file-output") {
		fmt.Println("The following option combination is not currently supported: version + json-file-output")
		return SNYK_EXIT_CODE_ERROR
	} else {
		c.printVersion()
		return SNYK_EXIT_CODE_OK
	}
}

func determineHandler(passthroughArgs []string) Handler {
	result := V1_DEFAULT

	if utils.Contains(passthroughArgs, "--version") ||
		utils.Contains(passthroughArgs, "-v") ||
		utils.Contains(passthroughArgs, "version") {
		result = V2_VERSION
	}

	return result
}

func PrepareV1EnvironmentVariables(input []string, integrationName string, integrationVersion string, proxyAddress string, caCertificateLocation string) (result []string, err error) {
	inputAsMap := utils.ToKeyValueMap(input, "=")
	result = input

	_, integrationNameExists := inputAsMap[SNYK_INTEGRATION_NAME_ENV]
	_, integrationVersionExists := inputAsMap[SNYK_INTEGRATION_VERSION_ENV]

	if !integrationNameExists && !integrationVersionExists {
		inputAsMap[SNYK_INTEGRATION_NAME_ENV] = integrationName
		inputAsMap[SNYK_INTEGRATION_VERSION_ENV] = integrationVersion
	} else if !(integrationNameExists && integrationVersionExists) {
		err = EnvironmentWarning{message: fmt.Sprintf("Partially defined environment, please ensure to provide both %s and %s together!", SNYK_INTEGRATION_NAME_ENV, SNYK_INTEGRATION_VERSION_ENV)}
	}

	if err == nil {
		// apply blacklist: ensure that no existing no_proxy or other configuration causes redirecting internal communication that is meant to stay between cliv1 and cliv2
		blackList := []string{
			SNYK_HTTPS_PROXY_ENV,
			SNYK_HTTP_PROXY_ENV,
			SNYK_CA_CERTIFICATE_LOCATION_ENV,
			SNYK_HTTP_NO_PROXY_ENV,
			SNYK_NPM_NO_PROXY_ENV,
			SNYK_NPM_HTTPS_PROXY_ENV,
			SNYK_NPM_HTTP_PROXY_ENV,
			SNYK_NPM_PROXY_ENV,
			SNYK_NPM_ALL_PROXY,
		}

		for _, key := range blackList {
			inputAsMap = utils.Remove(inputAsMap, key)
		}

		// fill expected values
		inputAsMap[SNYK_HTTPS_PROXY_ENV] = proxyAddress
		inputAsMap[SNYK_HTTP_PROXY_ENV] = proxyAddress
		inputAsMap[SNYK_CA_CERTIFICATE_LOCATION_ENV] = caCertificateLocation

		result = utils.ToSlice(inputAsMap, "=")
	}

	return result, err

}

func PrepareV1Command(cmd string, args []string, proxyPort int, caCertLocation string, integrationName string, integrationVersion string) (snykCmd *exec.Cmd, err error) {
	proxyAddress := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)

	snykCmd = exec.Command(cmd, args...)
	snykCmd.Env, err = PrepareV1EnvironmentVariables(os.Environ(), integrationName, integrationVersion, proxyAddress, caCertLocation)
	snykCmd.Stdin = os.Stdin
	snykCmd.Stdout = os.Stdout
	snykCmd.Stderr = os.Stderr

	return snykCmd, err
}

func (c *CLI) executeV1Default(wrapperProxyPort int, fullPathToCert string, passthroughArgs []string) int {
	c.DebugLogger.Println("launching snyk with path: ", c.v1BinaryLocation)
	c.DebugLogger.Println("fullPathToCert:", fullPathToCert)

	snykCmd, err := PrepareV1Command(
		c.v1BinaryLocation,
		passthroughArgs,
		wrapperProxyPort,
		fullPathToCert,
		c.GetIntegrationName(),
		c.GetFullVersion(),
	)

	if err != nil {
		if evWarning, ok := err.(EnvironmentWarning); ok {
			fmt.Println("WARNING! ", evWarning)
		}
	}

	err = snykCmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			return exitCode
		} else {
			// got an error but it's not an ExitError
			fmt.Println(err)
			return SNYK_EXIT_CODE_ERROR
		}
	}

	return SNYK_EXIT_CODE_OK
}

func (c *CLI) Execute(wrapperProxyPort int, fullPathToCert string, passthroughArgs []string) int {
	c.DebugLogger.Println("passthroughArgs", passthroughArgs)

	maybeMatchingBuiltinHandler := c.matchBuiltInHandler(passthroughArgs)
	if maybeMatchingBuiltinHandler != nil {
		c.DebugLogger.Println("matched built-in handler for: ", passthroughArgs)
		return maybeMatchingBuiltinHandler.Execute(wrapperProxyPort, fullPathToCert, passthroughArgs)
	}

	maybeMatchingExtension := matchExtension(passthroughArgs, c.Extensions)
	if maybeMatchingExtension != nil {
		matchedExtension := maybeMatchingExtension
		c.DebugLogger.Println("matched extension:", matchedExtension)

		matchedCommand, _, err := c.ArgParserRootCmd.Find(passthroughArgs)
		if err != nil {
			fmt.Println("There was an error with c.ArgParserRootCmd.Find()")
			return exit_codes.SNYK_EXIT_CODE_ERROR
		}

		extensionInput := MakeExtensionInput(matchedExtension.Metadata, matchedCommand, passthroughArgs, c.debugMode, wrapperProxyPort)
		if err != nil {
			fmt.Println(err)
			return exit_codes.SNYK_EXIT_CODE_ERROR
		}
		utils.PrettyLogObject(extensionInput, c.DebugLogger)
		return LaunchExtension(matchedExtension, extensionInput, wrapperProxyPort, fullPathToCert, c.DebugLogger)
	}

	c.DebugLogger.Println("No matching built-in handlers or extensions. Falling back on CLIv1")

	// fall-back on CLIv1
	return c.executeV1Default(wrapperProxyPort, fullPathToCert, passthroughArgs)
}

func (e EnvironmentWarning) Error() string {
	return e.message
}

type CommandHandler interface {
	Execute(wrapperProxyPort int, fullPathToCert string, passthroughArgs []string) int
}

type VersionHandler struct {
	cli *CLI
}

func (v *VersionHandler) Execute(wrapperProxyPort int, fullPathToCert string, passthroughArgs []string) int {
	if utils.Contains(passthroughArgs, "--json-file-output") {
		fmt.Println("The following option combination is not currently supported: version + json-file-output")
		return exit_codes.SNYK_EXIT_CODE_ERROR
	} else {
		v.cli.printVersion()
		return exit_codes.SNYK_EXIT_CODE_OK
	}
}

// Will return a CommandHandler for a built-in command for a given command (or set of args) if one exists
// By "built-in command", we mean one implemented directly in CLIv2's Go code, not an extension or the wrapped CLIv1
func (c *CLI) matchBuiltInHandler(args []string) CommandHandler {
	if utils.Contains(args, "--version") ||
		utils.Contains(args, "-v") ||
		utils.Contains(args, "version") {
		return &VersionHandler{cli: c}
	}
	return nil
}

func matchExtension(args []string, extensions []*extension.Extension) *extension.Extension {
	if len(args) > 0 {
		maybeCommand := args[0]
		for _, x := range extensions {
			if x.Metadata.Command.Name == maybeCommand {
				return x
			}
		}
	}

	return nil
}

func LaunchExtension(extension *extension.Extension, extensionInput *cli_extension_lib_go.ExtensionInput, proxyPort int, caCertLocation string, debugLogger *log.Logger) int {
	debugLogger.Println("launching extension:", extension.Metadata.Name)

	extensionInputJsonBytes, err := json.Marshal(extensionInput)
	if err != nil {
		fmt.Println("Error deserializing ExtensionInput", err)
		return exit_codes.SNYK_EXIT_CODE_ERROR
	}

	debugLogger.Println("extension input:\n", string(extensionInputJsonBytes))
	debugLogger.Println("extension binary path:", extension.BinPath)

	_, err = os.Stat(extension.BinPath)
	if err != nil {
		fmt.Println("error: extension binary does not exist:", extension.BinPath)
		return exit_codes.SNYK_EXIT_CODE_ERROR
	}

	cmd := exec.Command(extension.BinPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HTTPS_PROXY=http://127.0.0.1:%d", proxyPort),
		fmt.Sprintf("NODE_EXTRA_CA_CERTS=%s", caCertLocation),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	buffer := bytes.Buffer{}
	buffer.Write(extensionInputJsonBytes)
	buffer.WriteString("\n\n")
	cmd.Stdin = &buffer

	cmd.Start()
	err = cmd.Wait()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			return exitCode
		} else {
			// got an error but it's not an ExitError
			fmt.Println("error launching extension:", err)
			return exit_codes.SNYK_EXIT_CODE_ERROR
		}
	}

	return exit_codes.SNYK_EXIT_CODE_OK
}
