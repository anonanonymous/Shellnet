const THRESHOLD = 5;
function httpGet (theUrl) {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", theUrl, false);
    xmlHttp.send( null );
    return JSON.parse(xmlHttp.responseText);
}

function copy_ele (eleid) {
    console.log(eleid);
    let copyText = document.getElementById(eleid);
    let input = document.getElementById("temp_input");
    input.value = copyText.textContent;
    input.select();
    document.execCommand("copy");
}
