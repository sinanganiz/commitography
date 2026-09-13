import { ApiError } from '../api/client';
import { sentence } from './format';

/** A follow-up the form can offer next to an error. */
export type StartErrorAction = 'allowShallow' | 'reload' | 'refreshJobs';

export interface StartErrorGuidance {
  title: string;
  detail: string;
  /** True when the problem is with the typed path, so the field is marked. */
  pathProblem: boolean;
  action?: StartErrorAction;
}

/**
 * Turns a rejected start request into guidance the user can act on. The
 * server's message is safe to show, but on its own it rarely says what to do
 * next, so each stable error code gets its own explanation.
 */
export function describeStartError(error: unknown): StartErrorGuidance {
  if (!(error instanceof ApiError)) {
    return {
      title: 'The analysis could not be started.',
      detail: 'An unexpected error occurred in the page. Reload it and try again.',
      pathProblem: false,
      action: 'reload',
    };
  }

  switch (error.code) {
    case 'invalid_repository_path':
      return {
        title: 'Nothing was found at this path.',
        detail:
          'Check the spelling. The path is resolved on the machine running the server; in Docker, use the path ' +
          'inside the container (for example /repos/project), not the host path.',
        pathProblem: true,
      };
    case 'path_not_allowed':
      return {
        title: 'This path is outside the folders the server may read.',
        detail:
          `${sentence(error.message)} Restart commitography serve with --allowed-root pointing at a folder that ` +
          'contains this repository, including its Git directory for linked worktrees.',
        pathProblem: true,
      };
    case 'invalid_repository':
      return {
        title: 'This folder is not a Git repository.',
        detail:
          'Point to the repository folder, the one that contains .git, and make sure Git can read it. In Docker, ' +
          'also check that the --mount source path exists: Docker Desktop mounts an empty folder in place of a ' +
          'mistyped one.',
        pathProblem: true,
      };
    case 'shallow_repository':
      return {
        title: 'This is a shallow clone, so its history is incomplete.',
        detail:
          'Fetch the full history with git fetch --unshallow for accurate totals, or allow a shallow analysis and ' +
          'accept that counts and first-commit dates will be understated.',
        pathProblem: true,
        action: 'allowShallow',
      };
    case 'active_job':
      return {
        title: 'Another analysis is already running.',
        detail: 'Only one analysis runs at a time. Wait for it to finish or cancel it, then start this one.',
        pathProblem: false,
        action: 'refreshJobs',
      };
    case 'invalid_session':
      return {
        title: 'The local session has expired.',
        detail: 'This usually means the server was restarted. Reload the page to start a new session.',
        pathProblem: false,
        action: 'reload',
      };
    case 'network_error':
      return {
        title: 'The local server could not be reached.',
        detail: 'Check that commitography serve is still running, then try again.',
        pathProblem: false,
      };
    default:
      if (error.status >= 500) {
        return {
          title: 'The server could not start the analysis.',
          detail: 'Check the terminal running commitography serve for details, then try again.',
          pathProblem: false,
        };
      }
      return {
        title: 'The server rejected the request.',
        detail: sentence(error.message),
        pathProblem: false,
      };
  }
}
