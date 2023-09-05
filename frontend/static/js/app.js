function getRandom(min, max) {
    min = Math.ceil(min);
    max = Math.floor(max);
    return Math.floor(Math.random() * (max - min + 1) + min);
}

async function getFortune(id) {
    const response = await fetch(`https://api.lnkphm.online/fortunes/${id}`);
    const fortune = await response.json();
    return fortune
}

async function loadFortune() {
    rand = getRandom(0, 10);
    fortune = await getFortune(rand);
    showFortune(fortune)
}

function showFortune(fortune) {
    fortuneElem = document.getElementById("fortune-text");
    fortuneElem.textContent = fortune.name;
}


window.addEventListener("load", loadFortune)
