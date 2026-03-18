function stable() {
    const arr = [];
    for (let i = 0; i < 1e6; i++) {
        arr.push({ a: 1, b: 2 });
    }
}

function unstable() {
    const arr = [];
    for (let i = 0; i < 1e6; i++) {
        let o = { a: 1 };
        o.b = 2;
        arr.push(o);
    }
}

function unstable2() {
    const arr = [];
    for (let i = 0; i < 1e6; i++) {
        const o = {};
        if (i % 2)
            o.a = 1; // shape branches
        else o.b = 2; // megamorphic IC
        arr.push(o);
    }
}

function bench(label, fn) {
    const t0 = performance.now();
    fn();
    const t1 = performance.now();
    console.log(label, (t1 - t0).toFixed(2), 'ms');
}

unstable();
unstable2();
stable();

bench('unstable', unstable);
bench('unstable2', unstable2);
bench('stable', stable);
