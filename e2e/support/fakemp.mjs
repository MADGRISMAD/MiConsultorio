// Fake Mercado Pago API (preferences, OAuth, Point terminals, orders, payments).
// Control endpoints used by tests: POST /__pay {id, ...payment}, POST /__order_paid {id|"last"}, GET /__refunds.
import http from 'node:http';

export function startFakeMp({ port }) {
  const payments = {};
  const orders = {};
  const refunds = [];
  let mode = 'STANDALONE';
  let n = 0;
  const send = (res, code, body) => {
    res.writeHead(code, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(body));
  };
  const server = http.createServer(async (req, res) => {
    let body = '';
    for await (const c of req) body += c;
    const u = new URL(req.url, 'http://x');
    const p = u.pathname;
    const origin = `http://localhost:${port}`;
    if (p === '/__pay') {
      const d = JSON.parse(body);
      payments[d.id] = d;
      return send(res, 200, {});
    }
    if (p === '/checkout/preferences') {
      n++;
      return send(res, 201, { id: 'pref-' + n, init_point: `${origin}/pay/${n}`, sandbox_init_point: `${origin}/pay/${n}` });
    }
    if (p === '/oauth/token') return send(res, 200, { access_token: 'tok', refresh_token: 'ref', expires_in: 9999999, user_id: 77 });
    if (p === '/terminals/v1/list') return send(res, 200, { data: { terminals: [{ id: 'PAX__SN9', operating_mode: mode, external_pos_id: 'CAJA-1' }] } });
    if (p === '/terminals/v1/setup') {
      mode = 'PDV';
      return send(res, 200, {});
    }
    if (p === '/v1/orders' && req.method === 'POST') {
      const id = 'ord-' + ++n;
      orders[id] = { id, status: 'created', amount: JSON.parse(body).transactions.payments[0].amount };
      return send(res, 201, { id, status: 'created' });
    }
    if (p.startsWith('/v1/orders/') && p.endsWith('/cancel')) {
      const o = orders[p.split('/')[3]];
      if (o) o.status = 'canceled';
      return send(res, 200, {});
    }
    if (p.startsWith('/v1/orders/') && p.endsWith('/refund')) {
      refunds.push(p);
      return send(res, 200, {});
    }
    if (p.startsWith('/v1/orders/')) {
      const o = orders[p.split('/')[3]];
      if (!o) return send(res, 404, { message: 'nf' });
      return send(res, 200, o.status === 'paid' ? { id: o.id, status: 'processed', transactions: { payments: [{ id: '5' + o.id.slice(4), amount: o.amount, paid_amount: o.amount }] } } : o);
    }
    if (p === '/__order_paid') {
      const d = JSON.parse(body);
      orders[d.id === 'last' ? Object.keys(orders).pop() : d.id].status = 'paid';
      return send(res, 200, {});
    }
    if (p === '/__refunds') return send(res, 200, refunds);
    if (p.startsWith('/v1/payments/search')) return send(res, 200, { results: Object.values(payments).filter((x) => x.external_reference === u.searchParams.get('external_reference')) });
    if (p.startsWith('/v1/payments/')) {
      const x = payments[p.split('/').pop()];
      return x ? send(res, 200, x) : send(res, 404, { message: 'nf' });
    }
    send(res, 404, {});
  });
  server.listen(port, '127.0.0.1');
  return server;
}
