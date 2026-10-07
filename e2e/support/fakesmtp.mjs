// Minimal SMTP server that stores every message in a text file (messages separated by "=====").
import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';

export function startFakeSmtp({ port, file }) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, '');
  const server = net.createServer((c) => {
    let inData = false;
    let buf = '';
    const say = (s) => c.write(s + '\r\n');
    say('220 fake');
    c.on('data', (d) => {
      buf += d.toString();
      for (;;) {
        if (inData) {
          const i = buf.indexOf('\r\n.\r\n');
          if (i < 0) return;
          fs.appendFileSync(file, buf.slice(0, i) + '\n=====\n');
          buf = buf.slice(i + 5);
          inData = false;
          say('250 queued');
          continue;
        }
        const i = buf.indexOf('\r\n');
        if (i < 0) return;
        const line = buf.slice(0, i).toUpperCase();
        buf = buf.slice(i + 2);
        if (line === 'DATA') {
          inData = true;
          say('354 go');
        } else if (line === 'QUIT') {
          say('221 bye');
          c.end();
        } else say('250 ok');
      }
    });
    c.on('error', () => {});
  });
  server.listen(port, '127.0.0.1');
  return server;
}
