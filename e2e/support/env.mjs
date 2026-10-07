// Shared by the stack launcher (plain Node) and the tests. Everything can be overridden from the environment.
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const e2eDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const repoDir = path.resolve(e2eDir, '..');
export const tmpDir = path.join(e2eDir, '.tmp');

export const PORT = Number(process.env.E2E_PORT || 8099);
export const BASE = `http://localhost:${PORT}`;
export const SMTP_PORT = Number(process.env.E2E_SMTP_PORT || 2526);
export const MP_PORT = Number(process.env.E2E_MP_PORT || 9199);
export const MAIL_FILE = process.env.E2E_MAIL_FILE || path.join(tmpDir, 'mails.txt');
export const DATABASE_URL = process.env.E2E_DATABASE_URL || 'postgres://caresia:caresia@localhost:5432/caresia_e2e?sslmode=disable';
export const STATIC_DIR = process.env.E2E_STATIC_DIR || path.join(repoDir, 'frontend', 'build');

// The clinic created by the server at start-up (CLINIC_ADMIN_*): plan Crecimiento, setup done.
export const CLINIC = { username: 'dra', password: 'clave-segura-1', email: 'dra@e2e.test', name: 'Dra. Ejemplo' };
