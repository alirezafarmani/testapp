document.getElementById('check').onclick = async () => {
    try {
        const res = await fetch('/health');
        const data = await res.json();
        document.getElementById('result').textContent = JSON.stringify(data, null, 2);
    } catch (err) {
        document.getElementById('result').textContent = 'Error: ' + err;
    }
};

document.getElementById('submit').onclick = async () => {
    const name = document.getElementById('name').value.trim();
    const valueRaw = document.getElementById('value').value;
    const value = parseInt(valueRaw);

    if (!name || isNaN(value)) {
        document.getElementById('itemResult').textContent = '❌ لطفاً نام و عدد معتبر وارد کنید';
        return;
    }

    try {
        const res = await fetch('/items', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, value })
        });

        if (!res.ok) {
            const errText = await res.text();
            throw new Error(`Server responded with ${res.status}: ${errText}`);
        }

        const data = await res.json();
        document.getElementById('itemResult').textContent = JSON.stringify(data, null, 2);
    } catch (err) {
        document.getElementById('itemResult').textContent = 'Error: ' + err.message;
    }
};

