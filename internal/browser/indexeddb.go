package browser

// indexDBJS opens a small IndexedDB database, writes a record, and reads it
// back — exercising the browser's IndexedDB storage path.
const indexDBJS = `new Promise((resolve, reject) => {
	const req = indexedDB.open('bcrl-bench', 1);
	req.onupgradeneeded = () => {
		const db = req.result;
		if (!db.objectStoreNames.contains('bench')) {
			db.createObjectStore('bench');
		}
	};
	req.onsuccess = () => {
		const db = req.result;
		const tx = db.transaction('bench', 'readwrite');
		const store = tx.objectStore('bench');
		store.put('1', 'key');
		const get = store.get('key');
		get.onsuccess = () => {
			if (get.result === '1') {
				db.close();
				resolve('ok');
			} else {
				db.close();
				reject('idb mismatch');
			}
		};
		get.onerror = () => { db.close(); reject('idb read error'); };
	};
	req.onerror = () => reject('idb open error');
})`
