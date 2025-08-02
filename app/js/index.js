import {enhanceEditor} from "./editor.js";
import {enhanceHeaderInput} from "./header.js";
import {enhanceResizablePanels} from "./resize.js";

const dynamicConfigTextarea = document.getElementById("dynamic-config-editor")
if (dynamicConfigTextarea) {
    enhanceEditor(dynamicConfigTextarea);
}

const headerTextarea = document.getElementById("headers")
if (headerTextarea) {
    enhanceHeaderInput(headerTextarea);
}

enhanceResizablePanels();
