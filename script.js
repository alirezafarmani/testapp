document.addEventListener('DOMContentLoaded', () => {
    const userForm = document.getElementById('userForm');
    const userResult = document.getElementById('userResult');

    const func1Btn = document.getElementById('func1Btn');
    const func1Result = document.getElementById('func1Result');

    const func2Btn = document.getElementById('func2Btn');
    const func2Result = document.getElementById('func2Result');

    const getUsersBtn = document.getElementById('getUsersBtn');
    const getUsersResult = document.getElementById('getUsersResult');
    const usersList = document.getElementById('usersList');

    const linkUser = document.getElementById('link-user');
    const linkUsers = document.getElementById('link-users');
    const linkFunc1 = document.getElementById('link-func1');
    const linkFunc2 = document.getElementById('link-func2');
    const linkMetrics = document.getElementById('link-metrics');

    linkUser.textContent = getApiUrl('user');
    linkUser.href = getApiUrl('user');

    linkUsers.textContent = getApiUrl('users');
    linkUsers.href = getApiUrl('users');

    linkFunc1.textContent = getApiUrl('func1');
    linkFunc1.href = getApiUrl('func1');

    linkFunc2.textContent = getApiUrl('func2');
    linkFunc2.href = getApiUrl('func2');

    linkMetrics.textContent = getApiUrl('metrics');
    linkMetrics.href = getApiUrl('metrics');

    async function callApi(endpoint, options = {}) {
        const url = getApiUrl(endpoint);
        try {
            const res = await fetch(url, {
                headers: { 'Content-Type': 'application/json' },
                ...options
            });
            const text = await res.text();
            try {
                const json = JSON.parse(text || '{}');
                return { ok: res.ok, status: res.status, data: json };
            } catch (e) {
                return { ok: false, status: res.status, error: 'Invalid JSON response', raw: text };
            }
        } catch (err) {
            return { ok: false, status: 0, error: err.message };
        }
    }

    userForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        userResult.textContent = 'Sending...';

        const payload = {
            first_name: document.getElementById('firstName').value,
            last_name: document.getElementById('lastName').value,
            age: Number(document.getElementById('age').value),
            marital_status: document.getElementById('maritalStatus').value === 'true'
        };

        const res = await callApi('user', {
            method: 'POST',
            body: JSON.stringify(payload)
        });

        if (res.ok && res.data && res.data.success) {
            userResult.textContent = `✅ User created: ${res.data.user_id || ''}`;
        } else {
            userResult.textContent = `❌ Error: ${res.error || (res.data && res.data.message) || 'Unknown error'}`;
        }
    });

    func1Btn.addEventListener('click', async () => {
        func1Result.textContent = 'Triggering Func1...';
        const res = await callApi('func1', { method: 'GET' });
        if (res.ok && res.data && res.data.success) {
            func1Result.textContent = `✅ ${res.data.message}`;
        } else {
            func1Result.textContent = `❌ Error: ${res.error || (res.data && res.data.message) || 'Unknown error'}`;
        }
    });

    func2Btn.addEventListener('click', async () => {
        func2Result.textContent = 'Triggering Func2...';
        const res = await callApi('func2', { method: 'GET' });
        if (res.ok && res.data && res.data.success) {
            func2Result.textContent = `✅ ${res.data.message}`;
        } else {
            func2Result.textContent = `❌ Error: ${res.error || (res.data && res.data.message) || 'Unknown error'}`;
        }
    });

    getUsersBtn.addEventListener('click', async () => {
        getUsersResult.textContent = 'Loading users...';
        usersList.innerHTML = '';

        const res = await callApi('users', { method: 'GET' });
        if (res.ok && res.data && res.data.success) {
            getUsersResult.textContent = `✅ Found ${res.data.count || 0} users`;
            const users = res.data.users || [];
            users.forEach(u => {
                const div = document.createElement('div');
                div.className = 'user-item';
                div.textContent = `${u.first_name} ${u.last_name} (age ${u.age}) - married: ${u.marital_status}`;
                usersList.appendChild(div);
            });
        } else {
            getUsersResult.textContent = `❌ Error: ${res.error || (res.data && res.data.message) || 'Unknown error'}`;
        }
    });
});

