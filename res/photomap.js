// When the window has finished loading create our google map below
google.maps.event.addDomListener(window, 'load', init);

function lat2merc(lat) {
    return 180.0 / Math.PI * Math.log(Math.tan(Math.PI/4 + lat * Math.PI/180.0/2.0));
}

function PhotoMapControl(controlDiv, map, spotsOverlay, photosOverlay) {
  var control = this;

  var photosShown = true;
  var spotsShown = true;

  controlDiv.className = "photomapcontrol";

  var spotsUI = document.createElement('div');
  spotsUI.style.borderTopLeftRadius = "3px";
  spotsUI.style.borderBottomLeftRadius = "3px";
  spotsUI.className = "photomapui";
  spotsUI.id = 'spotsUI';
  spotsUI.title = 'Click to toggle photos spots';
  controlDiv.appendChild(spotsUI);

  var spotsText = document.createElement('div');
  spotsText.style.fontWeight = "500";
  spotsText.className = "photomapuitext";
  spotsText.id = 'spotsText';
  spotsText.innerHTML = 'Spots';
  spotsUI.appendChild(spotsText);

  var photosUI = document.createElement('div');
  photosUI.style.borderTopRightRadius = "3px";
  photosUI.style.borderBottomRightRadius = "3px";
  photosUI.className = "photomapui";
  photosUI.id = 'photosUI';
  photosUI.title = 'Click to toggle photo icons';
  controlDiv.appendChild(photosUI);

  var photosText = document.createElement('div');
  photosText.style.fontWeight = "500";
  photosText.className = "photomapuitext";
  photosText.id = 'photosText';
  photosText.innerHTML = 'Photos';
  photosUI.appendChild(photosText);

  photosUI.addEventListener('click', function() {
    if (photosShown) {
      var i = map.overlayMapTypes.indexOf(photosOverlay);
      if (i != -1) {
        map.overlayMapTypes.removeAt(i);
      }
    } else {
      map.overlayMapTypes.push(photosOverlay);
    }
    photosShown = !photosShown;
    photosText.style.fontWeight = photosShown ? "500" : "400";
  });

  spotsUI.addEventListener('click', function() {
    if (spotsShown) {
      var i = map.overlayMapTypes.indexOf(spotsOverlay);
      if (i != -1) {
        map.overlayMapTypes.removeAt(i);
      }
    } else {
      // insert before photosOverlay, if shown
      var i = map.overlayMapTypes.indexOf(photosOverlay);
      if (i != -1) {
        map.overlayMapTypes.insertAt(i, spotsOverlay);
      } else {
        map.overlayMapTypes.push(spotsOverlay);
      }
    }
    spotsShown = !spotsShown;
    spotsText.style.fontWeight = spotsShown ? "500" : "400";
  });
}

function initMap(mapElement, bounds) {
  // Basic options for a simple Google Map
  // For more options see: https://developers.google.com/maps/documentation/javascript/reference#MapOptions
  var mapOptions = {
      minZoom: 3,
      scaleControl: true,

      // How you would like to style the map. 
      // This is where you would paste any style found on Snazzy Maps.
      styles: [{"featureType":"landscape","stylers":[{"saturation":-100},{"lightness":65},{"visibility":"on"}]},{"featureType":"poi","stylers":[{"saturation":-100},{"lightness":51},{"visibility":"simplified"}]},{"featureType":"road.highway","stylers":[{"saturation":-100},{"visibility":"simplified"}]},{"featureType":"road.arterial","stylers":[{"saturation":-100},{"lightness":30},{"visibility":"on"}]},{"featureType":"road.local","stylers":[{"saturation":-100},{"lightness":40},{"visibility":"on"}]},{"featureType":"transit","stylers":[{"saturation":-100},{"visibility":"simplified"}]},{"featureType":"administrative.province","stylers":[{"visibility":"off"}]},{"featureType":"water","elementType":"labels","stylers":[{"visibility":"on"},{"lightness":-25},{"saturation":-100}]},{"featureType":"water","elementType":"geometry","stylers":[{"hue":"#ffff00"},{"lightness":-25},{"saturation":-97}]}]
  };

  // Create the Google Map using our element and options defined above
  var map = new google.maps.Map(mapElement, mapOptions);
  map.fitBounds(bounds);

  var bounds, lastBounds;
  var markers = [];
  function clearMarkers(latLng) {
    for (var i = 0; i < markers.length; i++) {
      markers[i].setMap(null);
    }
    markers = [];
  }
  function hideGallery() {
    var mapElem = document.getElementById('map');
    var sidebar = document.getElementById('sidebar');
    sidebar.style.visibility = "hidden";
    mapElem.style.left = "0%";
    mapElem.style.width = "100%";
    google.maps.event.trigger(map, 'resize');
  }
  function showGallery(lat, lng) {
    var u = ['gallery.json?la=', lat, '&lo=', lng,
      '&zoom=', map.getZoom()].join('');
    getJSON(u, function(gal) {
      if (!gal || gal.length == 0) {
        hideGallery();
        return;
      }
      var mapElem = document.getElementById('map');
      var sidebar = document.getElementById('sidebar');
      sidebar.style.visibility = "visible";
      mapElem.style.left = "25%";
      mapElem.style.width = "75%";
      google.maps.event.trigger(map, 'resize');
      var thumbs = [];
      for (var i = 0; i < gal.length; i++) {
        thumbs.push(['<img src="thumb/' + gal[i] + '"/>'].join(''));
      }
      var thumbElem = document.getElementById('thumbs');
      thumbElem.innerHTML = thumbs.join('');
    });
  }
  map.addListener("bounds_changed", function() {
    bounds = map.getBounds();
    clearMarkers();
  });
  map.addListener("idle", function() {
    if (bounds && !bounds.equals(lastBounds)) {
      clearMarkers();
      lastBounds = bounds;
      var la0 = bounds.getSouthWest().lat();
      var lo0 = bounds.getSouthWest().lng();
      var la1 = bounds.getNorthEast().lat();
      var lo1 = bounds.getNorthEast().lng();
      var z = map.getZoom();
      var u = ['viewport.json?la0=', la0, '&lo0=', lo0,
        '&la1=', la1, '&lo1=', lo1, '&zoom=', z].join('');
      getJSON(u, function(vp) {
        if (!vp) return;
        var r = vp.radius;
        var c = vp.coords;
        var nc = c.length;
        for (var i = 0; i < nc; i+=2) {
          var lat = c[i];
          var lng = c[i+1];
          var marker = new google.maps.Circle({
            center: new google.maps.LatLng(lat, lng),
            // todo: calc proper radius from latitude
            radius: r / 360.0 * 2e7,
            fillOpacity: 0.0,
            strokeOpacity: 0.0,
            map: map
          });
          function sg(marker, lat, lng) {
            marker.addListener('click', function() {
              showGallery(lat, lng);
            });
          }
          sg(marker, lat, lng);
          markers.push(marker);
        }
      });
    }
  });
  map.addListener("click", function(e) {
    hideGallery();
  });
  map.addListener("zoom_changed", function() {
  });

  // init overlays
  var spotOverlay = new google.maps.ImageMapType({
    getTileUrl: function(coord, zoom) {
      return ['/tile/spot/', coord.x, '_', coord.y, '_', zoom].join('');
    },
    opacity: 0.5,
    tileSize: google.maps.Size(256, 256)
  });
  map.overlayMapTypes.push(spotOverlay);
  var photoOverlay = new google.maps.ImageMapType({
    getTileUrl: function(coord, zoom) {
      return ['/tile/photo/', coord.x, '_', coord.y, '_', zoom].join('');
    },
    tileSize: google.maps.Size(256, 256)
  });
  map.overlayMapTypes.push(photoOverlay);

  // coord map for debugging
  function CoordMapType(tileSize) {
    this.tileSize = tileSize;
  }

  CoordMapType.prototype.getTile = function(coord, zoom, ownerDocument) {
    var div = ownerDocument.createElement('div');
    div.innerHTML = coord;
    div.style.width = this.tileSize.width + 'px';
    div.style.height = this.tileSize.height + 'px';
    div.style.fontSize = '10';
    div.style.borderStyle = 'dashed';
    div.style.borderWidth = '1px';
    div.style.borderColor = '#050';
    return div;
  };

  var pmControlDiv = document.createElement('div');
  var pmControl = new PhotoMapControl(pmControlDiv, map, spotOverlay, photoOverlay);

  pmControlDiv.index = 1;
  pmControlDiv.style['padding-top'] = '10px';
  map.controls[google.maps.ControlPosition.TOP_CENTER].push(pmControlDiv);
}

function init() {
  // Get the HTML DOM element that will contain your map
  // We are using a div with id="map" seen below in the <body>
  var mapElement = document.getElementById('map');

  var TILESIZE = 256;

  // number of tiles visible on screen in x/y directions
  var sx = mapElement.clientWidth / TILESIZE;
  var sy = mapElement.clientHeight / TILESIZE;

  getJSON("bounds.json", function(b) {
    var lat = b.lat;
    var lng = b.long;
    var dlat = b.dlat/2;
    var dlng = b.dlong/2;
    initMap(mapElement, {
      north: lat+dlat,
      west: lng-dlng,
      east: lng+dlng,
      south: lat-dlat
    });
  })
}

function getJSON(url, success) {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
      if (xhr.readyState == 4) {
        if (xhr.status == 200) {
          success(JSON.parse(xhr.responseText));
        } else {
          console.error(xhr.statusText);
        }
      }
    };
    xhr.open("GET", url, true);
    xhr.send();
}
