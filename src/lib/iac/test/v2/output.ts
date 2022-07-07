import { Ora } from 'ora';
import { EOL } from 'os';
import { convertEngineToJsonResults } from './json';
import { SnykIacTestOutput } from './scan/results';

import { TestCommandResult } from '../../../../cli/commands/types';
import {
  formatIacTestFailures,
  getIacDisplayedIssues,
  IaCTestFailure,
  spinnerSuccessMessage,
} from '../../../formatters/iac-output';
import { formatSnykIacTestTestData } from '../../../formatters/iac-output';
import { jsonStringifyLargeObject } from '../../../json';
import { IacOrgSettings } from '../../../../cli/commands/test/iac/local-execution/types';
import { SnykIacTestError } from './errors';

export function buildOutput({
  scanResult,
  testSpinner,
  projectName,
  orgSettings,
  options,
}: {
  scanResult: SnykIacTestOutput;
  testSpinner?: Ora;
  projectName: string;
  orgSettings: IacOrgSettings;
  options: any;
}): TestCommandResult {
  if (scanResult.results) {
    testSpinner?.succeed(spinnerSuccessMessage);
  } else {
    testSpinner?.stop();
  }

  let response = '';

  const testData = formatSnykIacTestTestData(scanResult.results);
  response += EOL + getIacDisplayedIssues(testData.resultsBySeverity);

  if (scanResult.errors) {
    const testFailures: IaCTestFailure[] = scanResult.errors.map((error) => {
      const formattedError = new SnykIacTestError(error);
      // If we received an error without a path it means that the scan failed
      if (!error?.fields?.path) {
        throw formattedError;
      }
      return {
        filePath: error.fields!.path!,
        failureReason: formattedError.userMessage,
      };
    });
    response += EOL.repeat(2) + formatIacTestFailures(testFailures);
  }

  const jsonData = jsonStringifyLargeObject(
    convertEngineToJsonResults({
      results: scanResult,
      projectName,
      orgSettings,
    }),
  );

  if (options.json) {
    return TestCommandResult.createJsonTestCommandResult(
      jsonData,
      jsonData,
      '',
    );
  }

  return TestCommandResult.createHumanReadableTestCommandResult(
    response,
    jsonData,
    '',
  );
}
