export function enhanceHeaderInput(originalInput) {
    const container = document.createElement("div");

    const value = originalInput.value || "";
    const lines = value.split("\n");

    for (let line of lines) {
        const parts = line.split(":");

        if (parts.length !== 2) {
            continue
        }

        addHeader(originalInput, container,
            parts[0].trim(),
            parts[1].trim())
    }

    addHeader(originalInput, container);

    originalInput.parentNode.appendChild(container);
}

function removeHeader(originalInput, header) {
    const container = header.parentNode
    const wasLastChild = header === container.lastElementChild;

    container.removeChild(header);

    if (wasLastChild && container.children.length > 0) {
        const lastChildButton = container.lastElementChild.querySelector("button");

        lastChildButton.textContent = '+';

        // Close to remove the old event listener.
        const newLastChildButton = lastChildButton.cloneNode(true);
        newLastChildButton.addEventListener("click", () => addHeader(originalInput, container))

        lastChildButton.replaceWith(newLastChildButton);
    }
}

function addHeader(originalInput, container, name, value) {
    const header = document.createElement("div");

    header.id = `header-${container.children.length}`;
    header.className = "input-group";
    header.innerHTML = `
        <input type="text" value="${name || ''}" placeholder="Header name" />
        <input type="text" value="${value || ''}" placeholder="Header value" />
        <button class="minimal">+</button>
    `;

    const button = header.querySelector("button");
    button.addEventListener("click", (e) => {
        e.preventDefault();
        addHeader(originalInput, container);
    });

    const inputs = header.querySelectorAll("input");
    for (let input of inputs) {
        input.addEventListener("keyup", debounce(() => syncHeaders(originalInput, container), 100));
    }

    if (container.children.length > 0) {
        const lastChild = container.lastElementChild
        const lastChildButton = lastChild.querySelector("button");

        lastChildButton.textContent = 'x';

        // Close to remove the old event listener.
        const newLastChildButton = lastChildButton.cloneNode(true);
        newLastChildButton.addEventListener("click", (e) => {
            e.preventDefault();
            removeHeader(originalInput, lastChild);
            syncHeaders(originalInput, container);
        })

        lastChildButton.replaceWith(newLastChildButton);
    }

    container.appendChild(header);
}

function syncHeaders(originalInput, container) {
    const inputs = container.querySelectorAll("input");

    const values = [];

    let i = 0;
    for (let input of inputs) {
        if (i % 2 === 0) {
            values.push([]);
        }

        values[values.length - 1].push(input.value)

        i++;
    }

    originalInput.value = values
        .filter(value => value.length === 2 && (value[0] !== "" || value[1] !== ""))
        .map(value => `${value[0]}: ${value[1]}`)
        .join("\n");
}

function debounce(callback, wait) {
    let timeoutId = null;

    return (...args) => {
        window.clearTimeout(timeoutId);

        timeoutId = window.setTimeout(() => {
            callback.apply(null, args);
        }, wait);
    };
}
