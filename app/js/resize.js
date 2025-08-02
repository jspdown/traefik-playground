export function enhanceResizablePanels() {
    const resizableElements = document.querySelectorAll("[data-resizable]");

    for (let element of resizableElements) {
        const resizableValue = element.getAttribute("data-resizable");
        const [direction, position] = resizableValue.split(':');

        if (!["vertical", "horizontal"].includes(direction)) {
            console.error(`Unsupported direction "${direction}", ignoring`);
            continue;
        }

        const validPositions = direction === 'horizontal' ? ['left', 'right'] : ['top', 'bottom'];
        const handlePosition = position || (direction === 'horizontal' ? 'right' : 'bottom');

        if (!validPositions.includes(handlePosition)) {
            console.error(`Invalid position "${handlePosition}" for ${direction} resize`);
            continue;
        }

        const handle = createDragHandle(direction, handlePosition);
        element.appendChild(handle);

        handle.addEventListener('mousedown', (e) => initResize(e, element, direction, handlePosition));
    }
}

function initResize(event, element, direction, handlePosition) {
    event.preventDefault();

    const isHorizontal = direction === 'horizontal';
    const startPos = isHorizontal ? event.clientX : event.clientY;
    const startRect = element.getBoundingClientRect();
    const startSize = isHorizontal ? startRect.width : startRect.height;

    const style = element.style;

    // Immediately set the initial flex basis to prevent layout shift.
    style.flex = `0 0 ${startSize}px`;

    document.body.style.userSelect = 'none';
    document.body.style.cursor = isHorizontal ? 'ew-resize' : 'ns-resize';

    function onMouseMove(e) {
        const currentPos = isHorizontal ? e.clientX : e.clientY;
        let delta = currentPos - startPos;

        if ((isHorizontal && handlePosition === 'left') || (!isHorizontal && handlePosition === 'top')) {
            delta = -delta;
        }

        const newSize = Math.max(10, startSize + delta);
        style.flexBasis = `${newSize}px`;
    }

    function onMouseUp() {
        document.removeEventListener('mousemove', onMouseMove);
        document.removeEventListener('mouseup', onMouseUp);
        document.body.style.userSelect = '';
        document.body.style.cursor = '';

        // Maintain explicit size while preserving original flex behavior.
        style.flex = `0 0 ${style.flexBasis}`;
    }

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
}

function createDragHandle(direction, position) {
    const handle = document.createElement("div");

    handle.classList.add("drag-handle", direction, position);

    return handle;
}
