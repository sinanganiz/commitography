import { useState } from 'react';
import type { ComponentProps } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { jsonResponse } from '../test/sampleReport';
import { RepositoryForm, initialRepositoryForm } from './RepositoryForm';

afterEach(cleanup);

function Harness(props: Partial<ComponentProps<typeof RepositoryForm>>) {
  const [value, setValue] = useState(initialRepositoryForm);
  return (
    <RepositoryForm
      value={value}
      onChange={setValue}
      ready
      activeJob={null}
      onStarted={() => {}}
      onRefreshJobs={() => {}}
      {...props}
    />
  );
}

const typePath = (path: string) =>
  fireEvent.change(screen.getByLabelText(/Repository path/), { target: { value: path } });
const start = () => fireEvent.click(screen.getByRole('button', { name: /Start analysis|Starting/ }));

describe('RepositoryForm', () => {
  it('asks for a path instead of sending an empty one', () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    render(<Harness />);
    start();
    expect(screen.getByText('Enter the path of a repository on this machine.')).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('refuses an inverted date range before contacting the server', () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
    render(<Harness />);
    typePath('/repos/api');
    fireEvent.click(screen.getByRole('button', { name: /Advanced options/ }));
    fireEvent.change(screen.getByLabelText('Since'), { target: { value: '2025-06-01' } });
    fireEvent.change(screen.getByLabelText('Until'), { target: { value: '2025-01-01' } });
    expect(screen.getByText('Until must not be earlier than Since.')).toBeTruthy();
    start();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('warns before naming contributors, and says when they are pseudonymous', () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole('button', { name: /Advanced options/ }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Include per-author section/ }));
    expect(screen.getByText('This report will name contributors')).toBeTruthy();
    fireEvent.click(screen.getByRole('checkbox', { name: /Anonymize contributors/ }));
    expect(screen.getByText('Contributors appear under pseudonyms')).toBeTruthy();
  });

  it('sends one request however often Start is pressed', async () => {
    const fetchMock = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal('fetch', fetchMock);
    render(<Harness />);
    typePath('/repos/api');
    start();
    start();
    start();
    await waitFor(() => expect(screen.getByRole('button', { name: /Starting/ })).toHaveProperty('disabled', true));
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('blocks a new analysis while another is active', () => {
    render(
      <Harness
        activeJob={{ id: 'abc', status: 'running', repoName: 'web', createdAt: '', startedAt: null, finishedAt: null, warningCount: 0 }}
      />,
    );
    expect(screen.getByText('Analysis in progress')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Start analysis/ })).toHaveProperty('disabled', true);
  });

  it('turns a rejected path into guidance on the field', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: { code: 'invalid_repository_path', message: 'repository path does not exist' } }, 400)),
    );
    render(<Harness />);
    typePath('C:\\Users\\ana\\src\\api');
    start();
    await waitFor(() => expect(screen.getAllByText('Nothing was found at this path.').length).toBeGreaterThan(0));
    expect(screen.getByLabelText(/Repository path/).getAttribute('aria-invalid')).toBe('true');
  });

  it('offers to allow a shallow clone and ticks the option', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: { code: 'shallow_repository', message: 'shallow' } }, 400)));
    render(<Harness />);
    typePath('/repos/api');
    start();
    fireEvent.click(await screen.findByRole('button', { name: 'Allow shallow' }));
    expect(screen.getByRole('checkbox', { name: /Allow shallow clone/ })).toHaveProperty('checked', true);
  });

  it('reports the started job', async () => {
    const onStarted = vi.fn();
    const fetchMock = vi.fn(async () => jsonResponse({ id: '0123456789abcdef', status: 'queued' }, 202));
    vi.stubGlobal('fetch', fetchMock);
    render(<Harness onStarted={onStarted} />);
    typePath('  /repos/api  ');
    start();
    await waitFor(() => expect(onStarted).toHaveBeenCalledWith({ id: '0123456789abcdef', status: 'queued' }));
    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(JSON.parse(String(init.body)).repoPath).toBe('/repos/api');
  });
});
