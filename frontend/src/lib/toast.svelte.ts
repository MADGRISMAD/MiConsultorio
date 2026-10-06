export interface Toast {
  id: number;
  kind: 'success' | 'error';
  message: string;
}

class ToastStore {
  items = $state<Toast[]>([]);
  #next = 1;

  show(message: string, kind: Toast['kind'] = 'success', ms = 3500) {
    const id = this.#next++;
    this.items = [...this.items, { id, kind, message }];
    setTimeout(() => this.dismiss(id), ms);
  }

  dismiss(id: number) {
    this.items = this.items.filter((t) => t.id !== id);
  }
}

export const toast = new ToastStore();
