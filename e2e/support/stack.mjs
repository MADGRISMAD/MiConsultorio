// Starts everything the suite needs and keeps running until it is stopped:
// resets the database, starts the fake SMTP and Mercado Pago servers, builds and starts the Go server.
// Playwright launches it through `webServer`; it can also be run by hand (`npm run stack`).
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { BASE, CLINIC, DATABASE_URL, MAIL_FILE, MP_PORT, PORT, SMTP_PORT, STATIC_DIR, repoDir, tmpDir } from './env.mjs';
import { startFakeMp } from './fakemp.mjs';
import { startFakeSmtp } from './fakesmtp.mjs';

fs.mkdirSync(tmpDir, { recursive: true });

if (!fs.existsSync(path.join(STATIC_DIR, 'index.html'))) {
  console.error(`No existe ${STATIC_DIR}/index.html: construye el frontend antes (npm run build --prefix frontend).`);
  process.exit(1);
}

// Fresh database every run: the suite assumes an empty one.
const reset = spawnSync('psql', [DATABASE_URL, '-v', 'ON_ERROR_STOP=1', '-qc', 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'], { stdio: 'inherit' });
if (reset.status !== 0) {
  console.error('No se pudo reiniciar la base de pruebas (¿existe y está PostgreSQL arriba?).');
  process.exit(1);
}

const smtp = startFakeSmtp({ port: SMTP_PORT, file: MAIL_FILE });
const mp = startFakeMp({ port: MP_PORT });

let bin = process.env.E2E_SERVER_BIN;
if (!bin) {
  bin = path.join(tmpDir, 'caresia-server');
  const build = spawnSync('go', ['build', '-o', bin, './cmd/server'], { cwd: path.join(repoDir, 'backend'), stdio: 'inherit' });
  if (build.status !== 0) process.exit(1);
}

const server = spawn(bin, [], {
  stdio: 'inherit',
  env: {
    ...process.env,
    DATABASE_URL,
    JWT_SECRET: 'e2e-secret-e2e-secret-e2e-secret-0123456789',
    COOKIE_SECURE: 'false',
    STATIC_DIR,
    ADDR: `:${PORT}`,
    APP_URL: BASE,
    API_PUBLIC_URL: BASE,
    SMTP_HOST: '127.0.0.1',
    SMTP_PORT: String(SMTP_PORT),
    MAIL_FROM: 'Caresia <no-reply@caresia.test>',
    MP_API_BASE: `http://localhost:${MP_PORT}`,
    MP_ACCESS_TOKEN: 'platform',
    MP_CLIENT_ID: 'cid',
    MP_CLIENT_SECRET: 'csec',
    CLINIC_NAME: 'Consultorio E2E',
    CLINIC_ADMIN_NAME: CLINIC.name,
    CLINIC_ADMIN_EMAIL: CLINIC.email,
    CLINIC_ADMIN_USERNAME: CLINIC.username,
    CLINIC_ADMIN_PASSWORD: CLINIC.password,
    TZ: 'America/Mexico_City'
  }
});

const stop = () => {
  server.kill('SIGTERM');
  smtp.close();
  mp.close();
  setTimeout(() => process.exit(0), 300);
};
process.on('SIGINT', stop);
process.on('SIGTERM', stop);
server.on('exit', (code) => {
  smtp.close();
  mp.close();
  process.exit(code ?? 1);
});
