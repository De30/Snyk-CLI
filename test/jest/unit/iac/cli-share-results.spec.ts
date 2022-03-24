import { shareResults } from '../../../../src/lib/iac/cli-share-results';
import {
  expectedEnvelopeFormatterResults,
  expectedEnvelopeFormatterResultsWithPolicy,
  createEnvelopeFormatterResultsWithTargetRef,
  scanResults,
} from './cli-share-results.fixtures';
import * as request from '../../../../src/lib/request/request';
import * as envelopeFormatters from '../../../../src/lib/iac/envelope-formatters';
import { Policy } from '../../../../src/lib/policy/find-and-load-policy';
import * as snykPolicyLib from 'snyk-policy';

describe('CLI Share Results', () => {
  let snykPolicy: Policy;
  let requestSpy, envelopeFormattersSpy;

  beforeAll(async () => {
    snykPolicy = await snykPolicyLib.load('test/jest/unit/iac/fixtures');
  });

  beforeEach(() => {
    requestSpy = jest
      .spyOn(request, 'makeRequest')
      .mockImplementation(async () => ({ res: {} as any, body: {} }));
    envelopeFormattersSpy = jest.spyOn(
      envelopeFormatters,
      'convertIacResultToScanResult',
    );
  });

  afterEach(() => {
    requestSpy.mockClear();
    envelopeFormattersSpy.mockClear();
  });

  it("converts the results to Envelope's ScanResult interface - without .snyk policies", async () => {
    await shareResults(scanResults, undefined);

    expect(envelopeFormattersSpy.mock.calls.length).toBe(2);

    const [firstCall, secondCall] = envelopeFormattersSpy.mock.calls;
    expect(firstCall[0]).toEqual(scanResults[0]);
    expect(secondCall[0]).toEqual(scanResults[1]);

    const [
      firstCallResult,
      secondCallResult,
    ] = envelopeFormattersSpy.mock.results;
    expect(firstCallResult.value).toEqual(expectedEnvelopeFormatterResults[0]);
    expect(secondCallResult.value).toEqual(expectedEnvelopeFormatterResults[1]);
  });

  it("converts the results to Envelope's ScanResult interface - with .snyk policies", async () => {
    await shareResults(scanResults, snykPolicy);

    expect(envelopeFormattersSpy.mock.calls.length).toBe(2);

    const [firstCall, secondCall] = envelopeFormattersSpy.mock.calls;
    expect(firstCall[0]).toEqual(scanResults[0]);
    expect(secondCall[0]).toEqual(scanResults[1]);

    const [
      firstCallResult,
      secondCallResult,
    ] = envelopeFormattersSpy.mock.results;
    expect(firstCallResult.value).toEqual(
      expectedEnvelopeFormatterResultsWithPolicy[0],
    );
    expect(secondCallResult.value).toEqual(
      expectedEnvelopeFormatterResultsWithPolicy[1],
    );
  });

  describe('when given a target reference', () => {
    it("should include it in the Envelope's ScanResult interface", async () => {
      const testTargetRef = 'test-target-ref';
      const expectedEnvelopeFormatterResults = createEnvelopeFormatterResultsWithTargetRef(
        testTargetRef,
      );

      await shareResults(scanResults, undefined, {
        'target-reference': testTargetRef,
      });

      expect(envelopeFormattersSpy.mock.calls.length).toBe(2);

      const [firstCall, secondCall] = envelopeFormattersSpy.mock.calls;
      expect(firstCall[0]).toEqual(scanResults[0]);
      expect(secondCall[0]).toEqual(scanResults[1]);

      const [
        firstCallResult,
        secondCallResult,
      ] = envelopeFormattersSpy.mock.results;
      expect(firstCallResult.value).toEqual(
        expectedEnvelopeFormatterResults[0],
      );

      expect(secondCallResult.value).toEqual(
        expectedEnvelopeFormatterResults[1],
      );
    });
  });

  it("converts the results to Envelope's ScanResult interface - with .snyk policies (2)", async () => {
    await shareResults(scanResults, snykPolicy);

    expect(envelopeFormattersSpy.mock.calls.length).toBe(2);

    const [firstCall, secondCall] = envelopeFormattersSpy.mock.calls;
    expect(firstCall[0]).toEqual(scanResults[0]);
    expect(secondCall[0]).toEqual(scanResults[1]);

    const [
      firstCallResult,
      secondCallResult,
    ] = envelopeFormattersSpy.mock.results;
    expect(firstCallResult.value).toEqual(
      expectedEnvelopeFormatterResultsWithPolicy[0],
    );
    expect(secondCallResult.value).toEqual(
      expectedEnvelopeFormatterResultsWithPolicy[1],
    );
  });

  it('forwards value to iac-cli-share-results endpoint', async () => {
    await shareResults(scanResults, undefined);

    expect(requestSpy.mock.calls.length).toBe(1);

    expect(requestSpy.mock.calls[0][0]).toMatchObject({
      method: 'POST',
      url: expect.stringContaining('/iac-cli-share-results'),
      json: true,
      headers: expect.objectContaining({
        authorization: expect.stringContaining('token'),
      }),
    });
  });

  it('respects the org flag provided', async () => {
    await shareResults(scanResults, undefined, {
      org: 'my-custom-org',
    });

    expect(requestSpy.mock.calls.length).toBe(1);

    expect(requestSpy.mock.calls[0][0]).toMatchObject({
      method: 'POST',
      url: expect.stringContaining('/iac-cli-share-results'),
      qs: expect.objectContaining({ org: 'my-custom-org' }),
      headers: expect.objectContaining({
        authorization: expect.stringContaining('token'),
      }),
    });
  });
});
